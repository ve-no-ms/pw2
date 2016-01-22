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
	"strings"

	"github.com/golang/glog"
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

func (d *Database) GenerateWebGPGUser(password []byte) (err error) {
	f, err := ioutil.TempFile("", "pw2")
	if err != nil {
		return
	}

	defer func() {
		err = os.Remove(f.Name())
	}()

	if _, err = fmt.Fprintf(f, `
%echo generating web interface gpg key...
Key-Type: RSA
Key-Length: 2048
Name-Real: pw2
Name-Email: `+gpgUserEmail+`
Name-Comment: this is the GPG key for the PW2 web interface
Passphrase: %s`, password); err != nil {
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

func (d *Database) gitCmd(arguments ...string) (c *exec.Cmd) {
	c = cmd("git", arguments...)

	c.Dir = d.Path

	return
}

func Create(location string, webPassword []byte) (d Database, err error) {
	d.Path = location
	// make sure the git repo folder exists
	if err = os.Mkdir(location, defaultMode); err != nil {
		return
	}

	// make the actual git repo
	if err = d.gitCmd("init").Run(); err != nil {
		return
	}

	if err = d.gitCmd("submodule", "add", blackboxSubmoduleLocation, "blackbox").Run(); err != nil {
		return
	}

	if err = d.gitCmd("commit", "-m", "add blackbox submodule").Run(); err != nil {
		return
	}

	if err = d.BlackboxCommand("initialize", "yes").Run(); err != nil {
		return
	}

	if err = d.gitCmd("add", "--", "keyrings", ".gitignore").Run(); err != nil {
		return
	}

	if err = d.gitCmd("commit", "-m", "🔒 initialize blackbox 🔒").Run(); err != nil {
		return
	}

	if webPassword == nil {
		if err = d.GenerateWebGPGUser(webPassword); err != nil {
			return
		}

		webPassword = nil

		if err = d.BlackboxCommand("addadmin", gpgUserEmail, path.Join("..", gpgHomedir)).
			Run(); err != nil {
			return
		}

		if err = d.gitCmd("add", "--",
			"keyrings/live/pubring.kbx",
			"keyrings/live/trustdb.gpg",
			"keyrings/live/blackbox-admins.txt").Run(); err != nil {
			return
		}

		if err = d.gitCmd("commit", "-m", "+ add web client as blackbox admin").Run(); err != nil {
			return
		}

	}

	return
}

//Func open opens a directory as a pw2 Database.
func Open(location string) (d Database, err error) {
	d.Path = location

	return
}

var Handler http.Handler
