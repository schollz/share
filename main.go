package main

//go:generate go get -v github.com/jteeuwen/go-bindata/go-bindata
//go:generate go-bindata static/

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"hash/fnv"
	"html/template"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/avct/uasurfer"
	"github.com/h2non/filetype"
	"github.com/hako/durafmt"
	log "github.com/schollz/logger"
)

// global config
var c Config

// Config contains all the configurable parameters
// for the server
type Config struct {
	PublicURL            string
	ContentDirectory     string
	Debug                bool
	Port                 string
	MaxBytesTotal        int64
	MaxBytesPerFile      int64
	MaxBytesPerFileHuman string
	MinutesPerGigabyte   int
}

// uploads keep track of parallel chunking
var uploadsLock sync.Mutex
var uploadsInProgress map[string]int
var uploadsFileNames map[string]string
var uploadsHashLock sync.Mutex
var uploadsHash map[string]string

// global tepmlate
var indexTemplate *template.Template

func main() {
	// flag ariables
	flag.StringVar(&c.ContentDirectory, "data", "data", "data directory")
	flag.StringVar(&c.PublicURL, "public", "", "public URL to use")
	flag.StringVar(&c.Port, "port", "8222", "port to use")
	flag.BoolVar(&c.Debug, "debug", false, "debug mode")
	flag.Int64Var(&c.MaxBytesPerFile, "max-file", 100000000, "max bytes per file")
	flag.Int64Var(&c.MaxBytesTotal, "max-total", 10000000000, "max bytes total")
	flag.IntVar(&c.MinutesPerGigabyte, "min-per-gig", 30, "number of minutes per gigabyte to scale auto-deletion")
	flag.Parse()

	// set a random seed for random activities
	rand.Seed(time.Now().UnixNano())

	// set debugging
	if c.Debug {
		log.SetLevel("debug")
	} else {
		log.SetLevel("info")
	}

	// initialize chunking maps
	uploadsInProgress = make(map[string]int)
	uploadsFileNames = make(map[string]string)
	uploadsHash = make(map[string]string)

	// initialize home page
	indexTemplate = template.New("basic")
	b, err := Asset("static/index.html")
	if err != nil {
		panic(err)
	}
	indexTemplate, err = indexTemplate.Parse(string(b))
	if err != nil {
		panic(err)
	}

	// initialize config
	c.MaxBytesPerFileHuman = HumanizeBytes(c.MaxBytesPerFile)
	if c.PublicURL == "" {
		c.PublicURL = "http://localhost:" + c.Port
	}
	os.Mkdir(c.ContentDirectory, os.ModePerm)

	// go routine for deleting old files
	go func() {
		for {
			deleteOld()
			time.Sleep(1 * time.Minute)
		}
	}()

	// start server
	log.Infof("Running on port %s", c.Port)
	http.HandleFunc("/", handler)
	http.ListenAndServe(":"+c.Port, nil)
}

// deleteOld goes through the files and deletes old uploads
func deleteOld() {
	dirSize, _, err := DirSize(c.ContentDirectory)
	if err != nil {
		log.Error(err)
	}

	// find all the meta informaiton
	files, err := filepath.Glob(fmt.Sprintf("%s/*/*.json.gz", c.ContentDirectory))
	if err != nil {
		log.Error(err)
		return
	}
	log.Debugf("found %d files, total %s", len(files), HumanizeBytes(dirSize))

	// go through each of the meta information files
	for _, f := range files {
		f = filepath.Clean(filepath.ToSlash(f))
		fsplit := strings.Split(f, "/")
		if len(fsplit) < 2 {
			continue
		}
		id := fsplit[1]
		p, err := loadPageInfo(id)
		if err != nil {
			log.Debugf("skipping %s: %s", id, err.Error())
			continue
		}
		if p.TimeToDeletion.Seconds() > 0 {
			continue
		}
		log.Debugf("deleting %s (%s, %s)", p.ID, p.SizeHuman, p.ModifiedHuman)
		err = os.RemoveAll(path.Join(c.ContentDirectory, p.ID))
		if err != nil {
			log.Error(err)
		}
	}
}

// TrimContent will continually purge things from the content directory until
// the content directoyr is below the specified size
func TrimContent() {
	for {
		dirSize, biggestFileID, err := DirSize(c.ContentDirectory)
		if err != nil {
			log.Error(err)
		}
		if dirSize < c.MaxBytesTotal || biggestFileID == "" {
			break
		}
		log.Debugf("bytes in directory exceeds max %d > %d", dirSize, c.MaxBytesTotal)
		log.Debugf("removing %s", biggestFileID)
		os.RemoveAll(path.Join(c.ContentDirectory, biggestFileID))
	}
}

// DirSize returns the size of a directory in bytes
func DirSize(path string) (int64, string, error) {
	var size int64
	biggestFileID := ""
	biggestFileSize := int64(0)
	err := filepath.Walk(path, func(pathName string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
			if info.Size() > biggestFileSize {
				pathName = filepath.ToSlash(pathName)
				biggestFileID = strings.Split(pathName, "/")[1]
				biggestFileSize = info.Size()
			}
		}
		return err
	})
	return size, biggestFileID, err
}

// handler is the main handler for all requests
func handler(w http.ResponseWriter, r *http.Request) {
	t := time.Now().UTC()
	err := handle(w, r)
	if err != nil {
		// an error has occured. return the home page if using a browser,
		// otherwise return a JSON response
		ua := uasurfer.Parse(r.Header.Get("User-Agent"))
		if ua.Browser.Name == uasurfer.BrowserUnknown {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"message": err.Error()})
		} else {
			p := NewPage()
			p.Error = err.Error()
			p.handleGetHome(w, r)
		}
	}
	log.Infof("%v %v %v %s", r.RemoteAddr, r.Method, r.URL.Path, time.Since(t))
}

// Page defines content that is available to each page
type Page struct {
	// properties of the file
	ID            string
	Name          string
	PathToFile    string
	Hash          string
	Link          string
	Size          int64
	SizeHuman     string
	ContentType   string
	Modified      time.Time
	ModifiedHuman string
	IsImage       bool
	IsText        bool
	IsAudio       bool
	IsVideo       bool

	// computed properties
	NameOnDisk          string
	Text                string
	TimeToDeletion      time.Duration
	TimeToDeletionHuman string

	// page specific info
	Key       string
	Error     string
	UserAgent *uasurfer.UserAgent

	// Config data
	Config Config
}

// NewPage returns a new page
func NewPage() (p *Page) {
	p = new(Page)
	p.Config = c
	return
}

// handlePut handles PUT requests from a command-line tool (wget or curl)
func (p *Page) handlePut(w http.ResponseWriter, r *http.Request) (err error) {
	fname, _ := filepath.Abs(r.URL.Path[1:])
	_, fname = filepath.Split(fname)
	if fname == "" {
		err = fmt.Errorf("No filename provided.")
		return err
	}
	p.Name, err = writeAllBytes(fname, r.Body)
	if err != nil {
		return
	}
	fmt.Fprint(w, c.PublicURL+"/"+p.Name+"\n")
	return nil
}

func (p *Page) handlePost(w http.ResponseWriter, r *http.Request) (err error) {
	r.ParseMultipartForm(32 << 20)
	file, handler, errForm := r.FormFile("file")
	if errForm != nil {
		err = errForm
		log.Error(err)
		return err
	}
	defer file.Close()
	fname, _ := filepath.Abs(handler.Filename)
	_, fname = filepath.Split(fname)

	log.Debugf("%+v", r.Form)
	chunkNum, _ := strconv.Atoi(r.FormValue("dzchunkindex"))
	chunkNum++
	totalChunks, _ := strconv.Atoi(r.FormValue("dztotalchunkcount"))
	chunkSize, _ := strconv.Atoi(r.FormValue("dzchunksize"))
	if int64(totalChunks)*int64(chunkSize) > c.MaxBytesPerFile {
		err = fmt.Errorf("Upload exceeds max file size: %s.", c.MaxBytesPerFileHuman)
		jsonResponse(w, http.StatusBadRequest, map[string]string{"message": err.Error()})
		return nil
	}
	uuid := r.FormValue("dzuuid")
	log.Debugf("working on chunk %d/%d for %s", chunkNum, totalChunks, uuid)

	f, err := ioutil.TempFile(c.ContentDirectory, "sharetemp")
	if err != nil {
		log.Error(err)
		return
	}
	// remove temp file when finished
	_, err = CopyMax(f, file, c.MaxBytesPerFile)
	f.Close()

	// check if need to cat
	uploadsLock.Lock()
	if _, ok := uploadsInProgress[uuid]; !ok {
		uploadsInProgress[uuid] = 0
	}
	uploadsInProgress[uuid]++
	uploadsFileNames[fmt.Sprintf("%s%d", uuid, chunkNum)] = f.Name()
	if uploadsInProgress[uuid] == totalChunks {
		err = func() (err error) {
			log.Debugf("upload finished for %s", uuid)
			log.Debugf("%+v", uploadsFileNames)
			delete(uploadsInProgress, uuid)

			fFinal, _ := ioutil.TempFile(c.ContentDirectory, "sharetemp")
			fFinalgz := gzip.NewWriter(fFinal)
			originalSize := int64(0)
			for i := 1; i <= totalChunks; i++ {
				// cat each chunk
				fh, err := os.Open(uploadsFileNames[fmt.Sprintf("%s%d", uuid, i)])
				delete(uploadsFileNames, fmt.Sprintf("%s%d", uuid, i))
				if err != nil {
					log.Error(err)
					return err
				}
				n, errCopy := io.Copy(fFinalgz, fh)
				originalSize += n
				if errCopy != nil {
					log.Error(errCopy)
				}
				fh.Close()
				log.Debugf("removed %s", fh.Name())
				os.Remove(fh.Name())
			}
			fFinalgz.Flush()
			fFinalgz.Close()
			fFinal.Close()
			log.Debugf("final written to: %s", fFinal.Name())
			fname, err = copyToContentDirectory(fname, fFinal.Name(), originalSize)

			log.Debugf("setting uploadsHash: %s", fname)
			uploadsHashLock.Lock()
			uploadsHash[uuid] = fname
			uploadsHashLock.Unlock()
			return
		}()
	}
	uploadsLock.Unlock()

	if err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"message": err.Error()})
		return nil
	}

	// wait until all are finished
	var finalFname string
	startTime := time.Now()
	for {
		uploadsHashLock.Lock()
		if _, ok := uploadsHash[uuid]; ok {
			finalFname = uploadsHash[uuid]
			log.Debugf("got uploadsHash: %s", finalFname)
		}
		uploadsHashLock.Unlock()
		if finalFname != "" {
			break
		}
		time.Sleep(100 * time.Millisecond)
		if time.Since(startTime).Seconds() > 60*60 {
			break
		}
	}

	// TODO: cleanup if last one, delete uuid from uploadshash

	jsonResponse(w, http.StatusCreated, map[string]string{"id": finalFname})
	return
}

func (p *Page) handleGetData(w http.ResponseWriter, r *http.Request, decompress bool) (err error) {
	f, err := os.Open(p.NameOnDisk)
	if err != nil {
		log.Error(err)
		return
	}
	if decompress {
		gzf, _ := gzip.NewReader(f)
		defer gzf.Close()
		io.Copy(w, gzf)
	} else {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", p.ContentType)
		io.Copy(w, f)
	}
	return
}

func (p *Page) handleShowDataInBrowser(w http.ResponseWriter, r *http.Request) (err error) {
	if p.IsText && p.Size < 20000 {
		b, _ := ioutil.ReadFile(p.NameOnDisk)
		gr, errGzip := gzip.NewReader(bytes.NewBuffer(b))
		if errGzip != nil {
			err = errGzip
			log.Error(err)
			return
		}
		defer gr.Close()
		var textBytes []byte
		textBytes, err = ioutil.ReadAll(gr)
		if err != nil {
			log.Error(err)
			return
		}

		p.Text = string(textBytes)
	}
	indexTemplate.Execute(w, p)
	return
}

func (p *Page) handleGetHome(w http.ResponseWriter, r *http.Request) (err error) {
	// https://astaxie.gitbooks.io/build-web-application-with-golang/en/04.5.html
	return indexTemplate.Execute(w, p)
}

// handle is the function for the main routing logic of share.
func handle(w http.ResponseWriter, r *http.Request) (err error) {
	// first get ID and filename if it is availble
	p := NewPage()

	if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/delete/") {
		// GET /delete/ID will delete the ID
		urlPathSplit := strings.Split(r.URL.Path, "/")
		id := urlPathSplit[len(urlPathSplit)-1]
		_, errStat := os.Stat(path.Join(c.ContentDirectory, id))
		if errStat != nil {
			err = fmt.Errorf("Data with id '%s' does not exist.", id)
			return
		}
		os.RemoveAll(path.Join(c.ContentDirectory, id))
		p := NewPage()
		p.Error = fmt.Sprintf("Removed %s.", id)
		return p.handleGetHome(w, r)
	} else if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/exists/") {
		urlPathSplit := strings.Split(r.URL.Path, "/")
		if len(urlPathSplit) < 3 {
			jsonResponse(w, http.StatusOK, map[string]string{"exists": "yes", "id": "", "name": ""})
		}
		id := filepath.Clean(urlPathSplit[len(urlPathSplit)-2])
		name := filepath.Clean(urlPathSplit[len(urlPathSplit)-1])
		_, errStat := os.Stat(path.Join(c.ContentDirectory, id, name))
		if errStat != nil {
			jsonResponse(w, http.StatusOK, map[string]string{"exists": "no", "id": id, "name": name})
		} else {
			jsonResponse(w, http.StatusOK, map[string]string{"exists": "yes", "id": id, "name": name})
		}
		return nil
	} else if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/static/") {
		// GET /static/<file> will return the <file> if it exists
		p.NameOnDisk = strings.TrimPrefix(filepath.ToSlash(filepath.Clean(r.URL.Path[1:])), "/") + ".gz"
		var b []byte
		b, err = Asset(p.NameOnDisk)
		if err != nil {
			log.Error(err)
			return
		}
		p.ContentType, err = GetFileContentTypeReader(p.NameOnDisk, bytes.NewBuffer(b))
		if err != nil {
			log.Error(err)
			return
		}
		log.Debugf("serving static file: %s (%s)", p.NameOnDisk, p.ContentType)
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", p.ContentType)
		_, err = w.Write(b)
		return
	} else if r.Method == "GET" && len(r.URL.Path) > 1 {
		// GET /<id> or /<id>/<filename> are the only other routes
		urlPath := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(r.URL.Path[1:])), "1/")
		log.Debugf("urlPath: %s", urlPath)
		urlPathSplit := strings.Split(urlPath, "/")
		id := urlPathSplit[0]
		var fname string
		if len(urlPathSplit) > 1 {
			fname = urlPathSplit[1]
		}
		_, errStat := os.Stat(path.Join(c.ContentDirectory, p.ID))
		if errStat != nil {
			err = fmt.Errorf("Data with id '%s' does not exist.", id)
			return
		}
		p, err = loadPageInfo(id)
		if err != nil {
			err = fmt.Errorf("Data with id '%s' does not exist.", id)
			return
		}

		if fname == "" || fname != p.Name {
			http.Redirect(w, r, fmt.Sprintf("/%s/%s", p.ID, p.Name), 302)
			return
		}
		_, errStat = os.Stat(path.Join(c.ContentDirectory, p.ID, p.Name))
		if errStat != nil {
			err = fmt.Errorf("Data with id '%s' does not exist.", id)
			return
		}
	}

	// set the config
	p.Config = c

	// generate key
	h := md5.New()
	io.WriteString(h, strconv.FormatInt(time.Now().Unix(), 10))
	p.Key = fmt.Sprintf("%x", h.Sum(nil))

	// get user agent information
	p.UserAgent = uasurfer.Parse(r.Header.Get("User-Agent"))

	// TODO: allow authorizatoin
	// // get authorization informaiton
	// auth := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
	// if len(auth) > 0 && auth[0] == "Basic" {
	// 	log.Info(auth)
	// 	payload, _ := base64.StdEncoding.DecodeString(auth[1])
	// 	pair := strings.SplitN(string(payload), ":", 2)
	// 	log.Debug(pair)
	// }

	if r.Method == "PUT" {
		// PUT file
		// this is called from curl/wget upload
		return p.handlePut(w, r)
	} else if r.Method == "POST" {
		// POST file
		// this is called from browser upload
		return p.handlePost(w, r)
	} else if r.Method == "GET" {
		if strings.HasPrefix(r.URL.Path, "/1/") {
			// GET /1/ID/<filename> will show the raw data
			return p.handleGetData(w, r, false)
		}
		if p.Name == "" || p.ID == "" {
			// GET /
			// show home page
			return p.handleGetHome(w, r)
		}
		// show data
		if p.UserAgent.Browser.Name == uasurfer.BrowserUnknown {
			// GET raw data and also decomppress it
			return p.handleGetData(w, r, true)
		} else {
			// GET /<id>/<filename> and show in browser
			return p.handleShowDataInBrowser(w, r)
		}
	}
	return
}

// loadPageInfo loads the meta information from the supplied ID
// and calculates information from it and returns the information as a Page type.
func loadPageInfo(id string) (p *Page, err error) {
	p = NewPage()
	f, err := os.Open(path.Join(c.ContentDirectory, id, id+".json.gz"))
	if err != nil {
		return
	}
	defer f.Close()

	w, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer w.Close()

	err = json.NewDecoder(w).Decode(&p)
	if err != nil {
		return nil, err
	}

	p.NameOnDisk = path.Join(c.ContentDirectory, p.ID, p.Name)
	p.TimeToDeletion = (time.Duration(c.MinutesPerGigabyte) * time.Minute) * time.Duration(1000000000/p.Size)
	p.TimeToDeletionHuman = durafmt.Parse(p.TimeToDeletion).String()
	p.ModifiedHuman = HumanizeTime(p.Modified)
	return
}

// writeAllBytes takes a reader and writes it to the content directory.
// It throws an error if the number of bytes written exceeds what is set.
func writeAllBytes(fname string, src io.Reader) (fnameFull string, err error) {
	f, err := ioutil.TempFile(c.ContentDirectory, "sharetemp")
	if err != nil {
		log.Error(err)
		return
	}
	// remove temp file when finished
	defer os.Remove(f.Name())
	w := gzip.NewWriter(f)

	// try to write the bytes
	n, err := CopyMax(w, src, c.MaxBytesPerFile)
	w.Flush()
	w.Close()
	f.Close()

	// if an error occured, then erase the temp file
	if err != nil {
		os.Remove(f.Name())
		log.Error(err)
		return
	} else {
		log.Debugf("wrote %d bytes to %s", n, f.Name())
	}
	return copyToContentDirectory(fname, f.Name(), n)
}

// copyToContentDirectory will move the temp file to the content directory and calculate
// the hash for generating the ID. It will also save the meta information in the content
// directory (the .json.gz files).
func copyToContentDirectory(fname string, tempFname string, originalSize int64) (fnameFull string, err error) {
	defer func() {
		os.Remove(tempFname)
		go TrimContent()
	}()

	hash, _ := Filemd5Sum(tempFname)
	// id := strings.ToLower(base32.StdEncoding.EncodeToString([]byte(hash)))[:8]
	id := RandStringBytesMaskImpr(hash, 6)
	// id := WordHash(hash)
	if _, err = os.Stat(path.Join(c.ContentDirectory, id)); !os.IsNotExist(err) {
		err = os.RemoveAll(path.Join(c.ContentDirectory, id))
		if err != nil {
			log.Error(err)
			return
		}
	}
	err = os.MkdirAll(path.Join(c.ContentDirectory, id), os.ModePerm)
	if err != nil {
		log.Error(err)
		return
	}
	err = os.Rename(tempFname, path.Join(c.ContentDirectory, id, fname))
	if err != nil {
		log.Error(err)
		return
	}
	log.Debugf("moved to %s", path.Join(id, fname))

	fnameFull = path.Join(id, fname)

	p := NewPage()
	p.ID = id
	p.Hash = hash
	p.Name = fname
	p.Size = originalSize
	p.SizeHuman = HumanizeBytes(originalSize)
	p.Modified = time.Now()
	p.ModifiedHuman = HumanizeTime(p.Modified)
	p.Link = fmt.Sprintf("/1/%s/%s", p.ID, p.Name)
	p.ContentType, err = GetFileContentType(path.Join(c.ContentDirectory, p.ID, p.Name))
	if err != nil {
		log.Error(err)
		return
	}
	p.IsImage = strings.Contains(p.ContentType, "image/")
	p.IsText = strings.Contains(p.ContentType, "text/")
	p.IsAudio = strings.Contains(p.ContentType, "audio/")
	p.IsVideo = strings.Contains(p.ContentType, "video/")

	// write gzipped JSON
	fWrite, err := os.Create(path.Join(c.ContentDirectory, id, id+".json.gz"))
	if err != nil {
		log.Error(err)
		return
	}
	defer fWrite.Close()
	wf := gzip.NewWriter(fWrite)
	defer wf.Close()
	enc := json.NewEncoder(wf)
	if err != nil {
		log.Error(err)
		return
	}
	enc.SetIndent("", " ")
	err = enc.Encode(p)
	if err != nil {
		log.Error(err)
		return
	}
	err = wf.Flush()

	return
}

// upload is a test function that can be used to upload content
func upload() {
	data, err := os.Open("text.txt")
	if err != nil {
		log.Error(err)
	}
	defer data.Close()
	req, err := http.NewRequest("PUT", "http://localhost:8080/test.txt", data)
	if err != nil {
		log.Error(err)
	}
	req.Header.Set("Content-Type", "text/plain")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		log.Error(err)
	}
	defer res.Body.Close()
}

// jsonResponse writes a JSON response and HTTP code
func jsonResponse(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json, err := json.Marshal(data)
	if err != nil {
		log.Error(err)
	}
	log.Debugf("json response: %s", json)
	fmt.Fprintf(w, "%s\n", json)
}

// Filemd5Sum determines the md5 hash of a file
func Filemd5Sum(pathToFile string) (result string, err error) {
	file, err := os.Open(pathToFile)
	if err != nil {
		return
	}
	defer file.Close()
	hash := md5.New()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		hash.Write(scanner.Bytes())
	}
	result = hex.EncodeToString(hash.Sum(nil))
	return
}

// GetFileContentType returns the MIME content-type of a file
func GetFileContentType(fname string) (contentType string, err error) {
	// Open a file descriptor
	file, err := os.Open(fname)
	if err != nil {
		return
	}
	defer file.Close()

	return GetFileContentTypeReader(fname, file)
}

// GetFileContentTypeReader returns the MIME content-type from the bytes and
// a file name.
func GetFileContentTypeReader(fname string, file io.Reader) (contentType string, err error) {
	gz, err := gzip.NewReader(file)
	if err != nil {
		return
	}
	defer gz.Close()

	// We only have to pass the file header = first 261 bytes
	head := make([]byte, 261)
	gz.Read(head)

	kind, err := filetype.Match(head)
	if err != nil {
		return
	}
	if kind == filetype.Unknown {
		contentType = strings.Split(http.DetectContentType(head), ";")[0]
		if contentType == "application/octet-stream" {
			if isASCII(string(head)) {
				contentType = "text/plain"
			}
		}
	} else {
		contentType = kind.MIME.Value
	}

	// if we have a text file, then use the filename to force what the
	// content type should be
	if contentType == "text/plain" {
		if strings.Contains(fname, ".js") {
			contentType = "application/javascript"
		} else if strings.Contains(fname, ".css") {
			contentType = "text/css"
		}
	}
	return
}

// isASCII returns whether a set of string-converted bytes
// are actually readable text.
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

// RandString prints a random string
func RandString(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}

// logn determines the log n
func logn(n, b float64) float64 {
	return math.Log(n) / math.Log(b)
}

// humanateBytes is a helper function for HumanizeBytes
func humanateBytes(s uint64, base float64, sizes []string) string {
	if s < 10 {
		return fmt.Sprintf("%d B", s)
	}
	e := math.Floor(logn(float64(s), base))
	suffix := sizes[int(e)]
	val := math.Floor(float64(s)/math.Pow(base, e)*10+0.5) / 10
	f := "%.0f %s"
	if val < 10 {
		f = "%.1f %s"
	}

	return fmt.Sprintf(f, val, suffix)
}

// HumanizeBytes produces a human readable representation of an SI size.
//
// See also: ParseBytes.
//
// HumanizeBytes(82854982) -> 83 MB
func HumanizeBytes(s int64) string {
	sizes := []string{"B", "kB", "MB", "GB", "TB", "PB", "EB"}
	return humanateBytes(uint64(s), 1000, sizes)
}

// Seconds-based time units
const (
	Day      = 24 * time.Hour
	Week     = 7 * Day
	Month    = 30 * Day
	Year     = 12 * Month
	LongTime = 37 * Year
)

// HumanizeTime formats a time into a relative string.
//
// HumanizeTime(someT) -> "3 weeks ago"
func HumanizeTime(then time.Time) string {
	return RelTime(then, time.Now(), "ago", "from now")
}

// A RelTimeMagnitude struct contains a relative time point at which
// the relative format of time will switch to a new format string.  A
// slice of these in ascending order by their "D" field is passed to
// CustomRelTime to format durations.
//
// The Format field is a string that may contain a "%s" which will be
// replaced with the appropriate signed label (e.g. "ago" or "from
// now") and a "%d" that will be replaced by the quantity.
//
// The DivBy field is the amount of time the time difference must be
// divided by in order to display correctly.
//
// e.g. if D is 2*time.Minute and you want to display "%d minutes %s"
// DivBy should be time.Minute so whatever the duration is will be
// expressed in minutes.
type RelTimeMagnitude struct {
	D      time.Duration
	Format string
	DivBy  time.Duration
}

var defaultMagnitudes = []RelTimeMagnitude{
	{time.Second, "now", time.Second},
	{2 * time.Second, "1 second %s", 1},
	{time.Minute, "%d seconds %s", time.Second},
	{2 * time.Minute, "1 minute %s", 1},
	{time.Hour, "%d minutes %s", time.Minute},
	{2 * time.Hour, "1 hour %s", 1},
	{Day, "%d hours %s", time.Hour},
	{2 * Day, "1 day %s", 1},
	{Week, "%d days %s", Day},
	{2 * Week, "1 week %s", 1},
	{Month, "%d weeks %s", Week},
	{2 * Month, "1 month %s", 1},
	{Year, "%d months %s", Month},
	{18 * Month, "1 year %s", 1},
	{2 * Year, "2 years %s", 1},
	{LongTime, "%d years %s", Year},
	{math.MaxInt64, "a long while %s", 1},
}

// RelTime formats a time into a relative string.
//
// It takes two times and two labels.  In addition to the generic time
// delta string (e.g. 5 minutes), the labels are used applied so that
// the label corresponding to the smaller time is applied.
//
// RelTime(timeInPast, timeInFuture, "earlier", "later") -> "3 weeks earlier"
func RelTime(a, b time.Time, albl, blbl string) string {
	return CustomRelTime(a, b, albl, blbl, defaultMagnitudes)
}

// CustomRelTime formats a time into a relative string.
//
// It takes two times two labels and a table of relative time formats.
// In addition to the generic time delta string (e.g. 5 minutes), the
// labels are used applied so that the label corresponding to the
// smaller time is applied.
func CustomRelTime(a, b time.Time, albl, blbl string, magnitudes []RelTimeMagnitude) string {
	lbl := albl
	diff := b.Sub(a)

	if a.After(b) {
		lbl = blbl
		diff = a.Sub(b)
	}

	n := sort.Search(len(magnitudes), func(i int) bool {
		return magnitudes[i].D > diff
	})

	if n >= len(magnitudes) {
		n = len(magnitudes) - 1
	}
	mag := magnitudes[n]
	args := []interface{}{}
	escaped := false
	for _, ch := range mag.Format {
		if escaped {
			switch ch {
			case 's':
				args = append(args, lbl)
			case 'd':
				args = append(args, diff/mag.DivBy)
			}
			escaped = false
		} else {
			escaped = ch == '%'
		}
	}
	return fmt.Sprintf(mag.Format, args...)
}

// CopyMax copies only the maxBytes and then returns an error if it
// copies equal to or greater than maxBytes (meaning that it did not
// complete the copy).
func CopyMax(dst io.Writer, src io.Reader, maxBytes int64) (n int64, err error) {
	n, err = io.CopyN(dst, src, maxBytes)
	if err != nil && err != io.EOF {
		return
	}

	if n >= maxBytes {
		err = fmt.Errorf("Upload exceeds maximum size (%s).", c.MaxBytesPerFileHuman)
	} else {
		err = nil
	}
	return
}

const letterBytes = "abcdefghijklmnopqrstuvwxyz0123456789"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

// RandStringBytesMaskImpr returns a random string of length *n* seeded by the
// hash of the supplied string *s*.
func RandStringBytesMaskImpr(s string, n int) string {
	h := fnv.New32a()
	h.Write([]byte(s))
	seed := int64(h.Sum32())

	src := rand.New(rand.NewSource(seed))

	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}
