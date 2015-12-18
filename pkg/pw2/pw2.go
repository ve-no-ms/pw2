// Package pw2 exposes an API for manipulating pw2 password databases
// and an http.Handler which exposes a pw2 server. More details can be found
// at the pw2 root godoc.
package pw2

import (
	"net/http"
	"os"

	git "github.com/libgit2/git2go"
)

const blackboxSubmoduleLocation = "https://github.com/StackExchange/blackbox.git"

type Database struct {
	Repo *git.Repository
}

func Create(location string) (d Database, err error) {
	// make sure the git repo folder exists
	if err = os.Mkdir(location, 0700); err != nil {
		return
	}

	// make the actual git repo
	if d.Repo, err = git.InitRepository(location, false); err != nil {
		return
	}

	// download / add the blackbox submodule TODO(does this need an existing dir)
	if _, err = d.Repo.Submodules.Add(blackboxSubmoduleLocation, "blackbox", false); err != nil {
		return
	}

	return
}

//Func DatabaseNotFound returns true if err is a git2go not found error ("object not found").
//This error would be returned for example when you're trying to load a database
//from a directory which doesn't yet exist.
func DatabaseNotFound(err error) bool {
	v, ok := err.(*git.GitError)

	return ok && v.Code == git.ErrNotFound
}

//Func open opens a directory as a pw2 Database.
func Open(location string) (d Database, err error) {
	if d.Repo, err = git.OpenRepository(location); err != nil {
		return
	}

	return
}

var Handler http.Handler
