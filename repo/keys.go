package repo

import (
	"crypto/rand"
	"github.com/libp2p/go-libp2p-crypto"
	"io/ioutil"
	"os"
	"path"
)

const PrivKeyFileName = "priv.key"

// Attempt to load the key from disk. If it doesn't exist let's create a new one.
func loadPrivKey(pth string) (crypto.PrivKey, error) {
	keyLocation := path.Join(pth, PrivKeyFileName)
	var privKey crypto.PrivKey
	// Check key file exists
	if _, err := os.Stat(keyLocation); os.IsNotExist(err) {
		// Here we're using the go-libp2p-crypto package to generate a new Ed25519 private key.
		// We could also use RSA here if we wanted or any other public key system for that matter
		// as long as we write an implementation that conforms to the libp2p crypto interface.
		privKey, _, err = crypto.GenerateEd25519Key(rand.Reader)
		if err != nil {
			return nil, err
		}

		// Marshal the private key for storage on disk
		keyBytes, err := crypto.MarshalPrivateKey(privKey)
		if err != nil {
			return nil, err
		}
		f, err := os.Create(keyLocation)
		if err != nil {
			return nil, err
		}
		// Write it to file
		f.Write(keyBytes)
		f.Close()
	} else {
		// Read the key bytes from disk
		keyBytes, err := ioutil.ReadFile(keyLocation)
		if err != nil {
			return nil, err
		}

		// Unmarshal it
		privKey, err = crypto.UnmarshalPrivateKey(keyBytes)
		if err != nil {
			return nil, err
		}
	}
	return privKey, nil
}
