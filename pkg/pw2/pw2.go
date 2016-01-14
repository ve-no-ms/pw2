// Package pw2 exposes an API for manipulating pw2 password databases
// and an http.Handler which exposes a pw2 server. More details can be found
// at the pw2 root godoc.
package pw2

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/golang/glog"
	git "github.com/libgit2/git2go"
)

const debug = true

const blackboxSubmoduleLocation = "https://github.com/StackExchange/blackbox.git"
const defaultMode = 0700
const gpgHomedir = "./gpg"
const gpgUserEmail = "fake@pw2.no.ms"

var workingDirectory string

func init() {
	var err error
	if workingDirectory, err = os.Getwd(); err != nil {
		panic(err)
	}
}

type ErrInexistant []string

func (e ErrInexistant) Error() string {
	return fmt.Sprintf("files do not exist: %s", strings.Join([]string(e), " "))
}

func assertExists(path ...string) (e error) {
	for _, p := range path {
		if _, err := os.Stat(p); err != nil {
			return ErrInexistant(path)
		}
	}

	return
}

type Database struct {
	Path string
	Repo *git.Repository
}

var pw2Signature = git.Signature{
	Name:  "pw2",
	Email: gpgUserEmail,
}

func gitSignature() (s *git.Signature) {
	so := pw2Signature
	so.When = time.Now()
	s = &so

	return
}

type cmdLogWriter struct {
	LogFunc func(fmt string, args ...interface{})
	Format  string
}

func (c cmdLogWriter) Write(b []byte) (n int, err error) {
	c.LogFunc(c.Format, b)
	return len(b), nil
}

var tabbedInfoLogger = cmdLogWriter{glog.V(3).Infof, "	%s"}
var tabbedErrorLogger = cmdLogWriter{glog.Errorf, "	%s"}

type ioFailer struct{ E error }

func (f ioFailer) Write(b []byte) (n int, err error) { return 0, f.E }
func (f ioFailer) Read(b []byte) (n int, err error)  { return 0, f.E }

var stdinBlock = ioFailer{errors.New("unexpected stdin read from *exec.Cmd")}

func cmd(command string, arguments ...string) (c *exec.Cmd) {
	glog.V(2).Infof("generate *exec.Cmd %s %s", command, strings.Join(arguments, " "))
	c = exec.Command(command, arguments...)

	c.Stderr = io.MultiWriter(tabbedErrorLogger, os.Stderr)
	c.Stdout = io.MultiWriter(tabbedInfoLogger, os.Stdout)

	c.Stdin = os.Stdin

	return
}

func (d *Database) GenerateWebGPGUser(password string) (err error) {
	f, err := ioutil.TempFile("", "pw2")
	if err != nil {
		return
	}

	defer func() {
		err = os.Remove(f.Name())
	}()

	if _, err = f.WriteString(`
%echo generating web interface gpg key...
Key-Type: RSA
Key-Length: 2048
Name-Real: pw2
Name-Email: ` + gpgUserEmail + `
Name-Comment: this is the GPG key for the PW2 web interface
Passphrase: ` + password); err != nil {
		return
	}

	if err = os.Mkdir("gpg", defaultMode); err != nil {
		return
	}

	if err = cmd("gpg", "--homedir", gpgHomedir, "--gen-key", "--batch", f.Name()).Run(); err != nil {
		return
	}

	return
}

func (d *Database) BlackboxCommand(command string, params ...string) (c *exec.Cmd) {
	c = cmd("bash", append([]string{path.Join("blackbox/bin/", "blackbox_"+command)}, params...)...)

	c.Dir = d.Path

	return
}

func (d *Database) commitIndex(message string) (oid *git.Oid, err error) {

	glog.V(2).Infof("~ git commit -m %+q", message)

	// yes, this is really how the library is meant to be used :(
	// https://github.com/libgit2/libgit2/blob/091165c53b2bcd5d41fb71d43ed5a23a3d96bf5d/tests/object/commit/commitstagedfile.c#L21-L134

	var index *git.Index
	if index, err = d.Repo.Index(); err != nil {
		return
	}

	defer index.Free()

	if glog.V(2) {
		glog.Infof("%v files in index", index.EntryCount())
	}

	var treeOid *git.Oid
	if treeOid, err = index.WriteTree(); err != nil {
		return
	}

	var tree *git.Tree
	if tree, err = d.Repo.LookupTree(treeOid); err != nil {
		return
	}

	var lastCommit *git.Commit
	if lastCommit, err = d.LastCommit(); err != nil {
		return
	}

	sig := gitSignature()

	switch lastCommit {
	case nil:
		if oid, err = d.Repo.CreateCommit(
			"HEAD",
			sig,
			sig,
			message,
			tree,
		); err != nil {
			return
		}
	default:
		defer lastCommit.Free()
		if oid, err = d.Repo.CreateCommit(
			"HEAD",
			sig,
			sig,
			message,
			tree,
			lastCommit,
		); err != nil {
			return
		}
	}

	return

}

func (d *Database) LastCommit() (c *git.Commit, err error) {
	// look up last commit
	var head *git.Reference
	if head, err = d.Repo.Head(); err != nil {
		fmt.Println(reflect.TypeOf(err))
		if gE, ok := err.(*git.GitError); ok && gE.Code == git.ErrUnbornBranch {
			err = nil
		}

		return

	}

	if c, err = d.Repo.LookupCommit(head.Target()); err != nil {
		return
	}

	return
}

func findGlob(base string, dirRecurse bool, pattern string) (matches []string, err error) {
	ms, err := filepath.Glob(filepath.Join(base, pattern))
	if err != nil {
		return
	}

	for _, m := range ms {
		if dirRecurse {
			var inf os.FileInfo
			inf, err = os.Stat(m)
			if err != nil {
				return
			}

			if inf.IsDir() {
				var dirMatches []string
				var rel string

				if rel, err = filepath.Rel(base, m); err != nil {
					return
				}

				dirMatches, err = findGlob(base, dirRecurse, filepath.Join(rel, "*"))
				if err != nil {
					return
				}

				matches = append(matches, dirMatches...)
				continue
			}
		}

		m, err = filepath.Rel(base, m)
		if err != nil {
			return
		}

		matches = append(matches, m)
	}

	return
}

type ErrUnmatched []string

func (e ErrUnmatched) Error() string {
	return fmt.Sprintf("expected %s to match files", strings.Join([]string(e), " "))
}

func findAllGlob(base string, mustMatchAll, dirRecurse bool, pattern ...string) (matches []string, err error) {
	var unmatchedPattern ErrUnmatched = make([]string, 0, len(pattern))
	for _, p := range pattern {
		var ms []string
		ms, err = findGlob(base, dirRecurse, p)
		if err != nil {
			return
		}

		if mustMatchAll && len(ms) < 1 {
			unmatchedPattern = append(unmatchedPattern, p)
		}

		matches = append(matches, ms...)
	}

	if len(unmatchedPattern) > 0 {
		err = unmatchedPattern
	}

	return
}

func (d *Database) addFilesGlob(patterns ...string) (err error) {
	files, err := findAllGlob("git", true, true, patterns...)
	if err != nil {
		return
	}

	return d.addFiles(files...)
}

func (d *Database) addFiles(files ...string) (err error) {
	glog.V(2).Infof("~ git add %s", strings.Join(files, " "))
	var index *git.Index
	if index, err = d.Repo.Index(); err != nil {
		return
	}

	for _, f := range files {
		if err = index.AddByPath(f); err != nil {
			return
		}
	}

	return
}

func Create(location string, webPassword string) (d Database, err error) {
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

	if err = d.BlackboxCommand("initialize", "yes").Run(); err != nil {
		return
	}

	if err = d.addFilesGlob("keyrings", ".gitignore"); err != nil {
		return
	}

	if _, err = d.commitIndex("🔒 initialize blackbox 🔒"); err != nil {
		return
	}

	if webPassword != "" {
		if err = d.GenerateWebGPGUser(webPassword); err != nil {
			return
		}

		if err = d.BlackboxCommand("addadmin", gpgUserEmail, path.Join("..", gpgHomedir)).
			Run(); err != nil {
			return
		}

		if err = d.addFiles(
			"keyrings/live/pubring.kbx",
			"keyrings/live/trustdb.gpg",
			"keyrings/live/blackbox-admins.txt"); err != nil {
			return
		}

		if _, err = d.commitIndex("+ add web client as blackbox admin"); err != nil {
			return
		}

	}

	return
}

func detachedHeadPull(r *git.Repository) (err error) {
	glog.Info("pulling remote...")
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

	glog.Info("remote pull complete")

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
