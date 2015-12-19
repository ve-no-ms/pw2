// Package pw2 exposes an API for manipulating pw2 password databases
// and an http.Handler which exposes a pw2 server. More details can be found
// at the pw2 root godoc.
package pw2

import (
	"net/http"
	"os"
	"os/exec"
	"path"
	"time"

	git "github.com/libgit2/git2go"
)

const debug = true

const blackboxSubmoduleLocation = "https://github.com/StackExchange/blackbox.git"
const defaultMode = 0700

type Database struct {
	Path string
	Repo *git.Repository
}

var pw2Signature = git.Signature{
	Name:  "pw2",
	Email: "N/A",
}

func gitSignature() (s *git.Signature) {
	so := pw2Signature
	so.When = time.Now()
	s = &so

	return
}

func (d *Database) BlackboxCommand(cmd string, params ...string) *exec.Cmd {
	return exec.Command("bash", append([]string{path.Join(d.Path, "blackbox/bin/", "blackbox_"+cmd)}, params...)...)
}

func (d *Database) commitIndex(message string) (oid *git.Oid, err error) {
	var index *git.Index
	if index, err = d.Repo.Index(); err != nil {
		return
	}

	var newHeadOid *git.Oid
	if newHeadOid, err = index.WriteTree(); err != nil {
		return
	}

	var newHeadTree *git.Tree
	if newHeadTree, err = d.Repo.LookupTree(newHeadOid); err != nil {
		return
	}

	sig := gitSignature()

	if oid, err = d.Repo.CreateCommit(
		"HEAD",
		sig,
		sig,
		message,
		newHeadTree,
	); err != nil {
		return
	}

	return

}

func (d *Database) addFiles(files ...string) (err error) {
	var index *git.Index
	if index, err = d.Repo.Index(); err != nil {
		return
	}

	return index.AddAll(
		files,
		git.IndexAddDefault,
		func(a, b string) int { return 0 },
	)

}

// Commits a number of files to the repo's HEAD. If `files` is empty, all files are committed.
func (d *Database) commitFiles(message string, files ...string) (oid *git.Oid, err error) {

	err = d.addFiles(files...)
	if err != nil {
		return
	}

	return d.commitIndex(message)
}
func Create(location string) (d Database, err error) {
	d.Path = location
	// make sure the git repo folder exists
	if err = os.Mkdir(location, defaultMode); err != nil {
		return
	}

	// make the actual git repo
	if d.Repo, err = git.InitRepository(location /*🐻*/, false); err != nil {
		return
	}

	// add the blackbox submodule
	var md *git.Submodule
	if md, err = d.Repo.Submodules.Add(blackboxSubmoduleLocation, "blackbox", false); err != nil {
		return
	}

	// get the repo of the new submodule
	var mdRepo *git.Repository
	if mdRepo, err = md.Open(); err != nil {
		return
	}

	// get the latest version of blackbox from the internart
	if err = detachedHeadPull(mdRepo); err != nil {
		return
	}

	// write the submodule to the superproject index
	if err = md.AddToIndex(true); err != nil {
		return
	}

	if err = d.addFiles(".gitmodules"); err != nil {
		return
	}

	if _, err = d.commitIndex("add blackbox submodule"); err != nil {
		return
	}

	_, err = d.BlackboxCommand("initialize", "yes").Output()
	if err != nil {
		return
	}

	if err = d.addFiles("keyrings", ".gitignore"); err != nil {
		return
	}

	if _, err = d.commitIndex("🔒 initialize blackbox 🔒"); err != nil {
		return
	}

	return
}

func detachedHeadPull(r *git.Repository) (err error) {
	var origin *git.Remote
	if origin, err = r.Remotes.Lookup("origin"); err != nil {
		return
	}

	if err = origin.Fetch([]string{}, &git.FetchOptions{}, ""); err != nil {
		return
	}

	if err = r.SetHead("refs/remotes/origin/master"); err != nil {
		return
	}

	var ref *git.Reference
	if ref, err = r.Head(); err != nil {
		return
	}

	var commit *git.Commit
	if commit, err = r.LookupCommit(ref.Target()); err != nil {
		return
	}

	if err = r.ResetToCommit(commit, git.ResetHard, &git.CheckoutOpts{
		Strategy: git.CheckoutForce,
		DirMode:  0700,
		FileMode: 0700,
	}); err != nil {
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
	d.Path = location

	if d.Repo, err = git.OpenRepository(location); err != nil {
		return
	}

	return
}

var Handler http.Handler
