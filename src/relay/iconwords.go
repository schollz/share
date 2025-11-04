package relay

import (
	"crypto/rand"
	"math/big"
)

// IconWord represents a Font Awesome icon and its associated word
type IconWord struct {
	Icon string
	Word string
}

// IconWords is a list of Font Awesome icons with their associated words
// This list matches the ICON_CLASSES array in web/src/App.jsx
var IconWords = []IconWord{
	{"fa-anchor", "anchor"},
	{"fa-apple-whole", "apple"},
	{"fa-atom", "atom"},
	{"fa-award", "award"},
	{"fa-basketball", "basketball"},
	{"fa-bell", "bell"},
	{"fa-bicycle", "bicycle"},
	{"fa-bolt", "bolt"},
	{"fa-bomb", "bomb"},
	{"fa-book", "book"},
	{"fa-box", "box"},
	{"fa-brain", "brain"},
	{"fa-briefcase", "briefcase"},
	{"fa-bug", "bug"},
	{"fa-cake-candles", "cake"},
	{"fa-calculator", "calculator"},
	{"fa-camera", "camera"},
	{"fa-campground", "campground"},
	{"fa-car", "car"},
	{"fa-carrot", "carrot"},
	{"fa-cat", "cat"},
	{"fa-chess-knight", "knight"},
	{"fa-chess-rook", "rook"},
	{"fa-cloud", "cloud"},
	{"fa-code", "code"},
	{"fa-gear", "gear"},
	{"fa-compass", "compass"},
	{"fa-cookie", "cookie"},
	{"fa-crow", "crow"},
	{"fa-cube", "cube"},
	{"fa-diamond", "diamond"},
	{"fa-dog", "dog"},
	{"fa-dove", "dove"},
	{"fa-dragon", "dragon"},
	{"fa-droplet", "droplet"},
	{"fa-drum", "drum"},
	{"fa-earth-americas", "earth"},
	{"fa-egg", "egg"},
	{"fa-envelope", "envelope"},
	{"fa-fan", "fan"},
	{"fa-feather", "feather"},
	{"fa-fire", "fire"},
	{"fa-fish", "fish"},
	{"fa-flag", "flag"},
	{"fa-flask", "flask"},
	{"fa-floppy-disk", "floppy"},
	{"fa-folder", "folder"},
	{"fa-football", "football"},
	{"fa-frog", "frog"},
	{"fa-gamepad", "gamepad"},
	{"fa-gavel", "gavel"},
	{"fa-gem", "gem"},
	{"fa-ghost", "ghost"},
	{"fa-gift", "gift"},
	{"fa-guitar", "guitar"},
	{"fa-hammer", "hammer"},
	{"fa-hat-cowboy", "cowboy"},
	{"fa-hat-wizard", "wizard"},
	{"fa-heart", "heart"},
	{"fa-helicopter", "helicopter"},
	{"fa-helmet-safety", "helmet"},
	{"fa-hippo", "hippo"},
	{"fa-horse", "horse"},
	{"fa-hourglass-half", "hourglass"},
	{"fa-snowflake", "snowflake"},
	{"fa-key", "key"},
	{"fa-leaf", "leaf"},
	{"fa-lightbulb", "lightbulb"},
	{"fa-magnet", "magnet"},
	{"fa-map", "map"},
	{"fa-microphone", "microphone"},
	{"fa-moon", "moon"},
	{"fa-mountain", "mountain"},
	{"fa-mug-hot", "mug"},
	{"fa-music", "music"},
	{"fa-paintbrush", "paintbrush"},
	{"fa-paper-plane", "paper"},
	{"fa-paw", "paw"},
	{"fa-pen", "pen"},
	{"fa-pepper-hot", "pepper"},
	{"fa-rocket", "rocket"},
	{"fa-road", "road"},
	{"fa-school", "school"},
	{"fa-screwdriver-wrench", "screwdriver"},
	{"fa-scroll", "scroll"},
	{"fa-seedling", "seedling"},
	{"fa-shield-heart", "shield"},
	{"fa-ship", "ship"},
	{"fa-skull", "skull"},
	{"fa-sliders", "sliders"},
	{"fa-splotch", "splotch"},
	{"fa-spider", "spider"},
	{"fa-star", "star"},
	{"fa-sun", "sun"},
	{"fa-toolbox", "toolbox"},
	{"fa-tornado", "tornado"},
	{"fa-tree", "tree"},
	{"fa-trophy", "trophy"},
	{"fa-truck", "truck"},
	{"fa-user-astronaut", "astronaut"},
	{"fa-wand-magic-sparkles", "wand"},
	{"fa-wrench", "wrench"},
	{"fa-pizza-slice", "pizza"},
	{"fa-burger", "burger"},
	{"fa-lemon", "lemon"},
}

// GenerateRandomIconMnemonic generates a mnemonic consisting of N random icon words
func GenerateRandomIconMnemonic(count int) string {
	if count <= 0 {
		count = 3
	}

	words := make([]string, count)
	for i := 0; i < count; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(IconWords))))
		if err != nil {
			// Fallback to a deterministic but acceptable index
			n = big.NewInt(int64(i % len(IconWords)))
		}
		words[i] = IconWords[n.Int64()].Word
	}

	return words[0] + "-" + words[1] + "-" + words[2]
}

// GenerateIconMnemonicFromID generates a deterministic mnemonic from a client ID
// using 3 icon words
func GenerateIconMnemonicFromID(clientID string) string {
	// Use hash-based selection to deterministically pick 3 icons
	hash := 0
	for _, ch := range clientID {
		hash = (hash*31 + int(ch)) & 0x7FFFFFFF
	}

	// Pick 3 different icons based on different parts of the hash
	idx1 := hash % len(IconWords)
	idx2 := (hash / len(IconWords)) % len(IconWords)
	idx3 := (hash / (len(IconWords) * len(IconWords))) % len(IconWords)

	// Ensure all three are different
	if idx2 == idx1 {
		idx2 = (idx2 + 1) % len(IconWords)
	}
	if idx3 == idx1 || idx3 == idx2 {
		idx3 = (idx3 + 1) % len(IconWords)
		if idx3 == idx1 {
			idx3 = (idx3 + 1) % len(IconWords)
		}
	}

	return IconWords[idx1].Word + "-" + IconWords[idx2].Word + "-" + IconWords[idx3].Word
}
