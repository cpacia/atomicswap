package repo

import (
	ds "github.com/ipfs/go-datastore"
	lvldb "github.com/ipfs/go-ds-leveldb"
	"github.com/libp2p/go-libp2p-crypto"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/mitchellh/go-homedir"
	"os"
	"path"
	"path/filepath"
	"runtime"
)

type Repo struct {
	pth            string
	dstore         ds.Batching
	privKey        crypto.PrivKey
	bootstrapPeers []pstore.PeerInfo
}

// Init a new repo. This will create the data directory and leveldb database if it doesn't exist.
// It will also attempt to load or create a new identity private key if the key file does not yet exist.
func NewRepo(pth string) (*Repo, error) {
	var err error
	// pth can be provided as an option to the start command
	// if the user did not provide one let's just use a default directory
	if pth == "" {
		pth, err = defaultRepoPath()
		if err != nil {
			return nil, err
		}
	}

	// If the directory doesn't exist, create it
	if _, err := os.Stat(pth); os.IsNotExist(err) {
		err := os.MkdirAll(path.Join(pth, "datastore"), os.ModePerm)
		if err != nil {
			return nil, err
		}
	}

	// Load or create the private key
	privkey, err := loadPrivKey(pth)
	if err != nil {
		return nil, err
	}

	// Create the leveldb datastore
	dstore, err := lvldb.NewDatastore(path.Join(pth, "datastore"), nil)
	if err != nil {
		return nil, err
	}

	bootstrapPeers, err := ParseBootstrapPeers(defaultBoostrapPeers)
	if err != nil {
		return nil, err
	}

	return &Repo{
		pth:            pth,
		privKey:        privkey,
		dstore:         dstore,
		bootstrapPeers: bootstrapPeers,
	}, nil
}

func (r *Repo) Path() string {
	return r.pth
}

func (r *Repo) PrivKey() crypto.PrivKey {
	return r.privKey
}

func (r *Repo) Datastore() ds.Batching {
	return r.dstore
}

func (r *Repo) BootstrapPeers() []pstore.PeerInfo {
	return r.bootstrapPeers
}

// Get the default data directory location
func defaultRepoPath() (string, error) {
	path := "~"
	directoryName := "atomicswaps"

	switch runtime.GOOS {
	case "linux":
		directoryName = ".atomicswaps"
	case "darwin":
		path = "~/Library/Application Support"
	}

	fullPath, err := homedir.Expand(filepath.Join(path, directoryName))
	if err != nil {
		return "", err
	}
	return filepath.Clean(fullPath), nil
}
