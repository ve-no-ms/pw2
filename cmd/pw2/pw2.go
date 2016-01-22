// Command pw2 provides the pw2 password manager HTTP service.
//
// pw2 stores passwords encrypted with GPG via StackExchange's blackbox
// tool, backed by git. Pw2 requires libgit2 and blackbox
// (https://github.com/StackExchange/blackbox) to be installed to function.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"reflect"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/golang/glog"
	"github.com/venoms/pw2/pkg/pw2"
)

const repoDir = "git"

var httpAddr string

var webInterface bool

func init() {
	flag.StringVar(
		&httpAddr,
		"http",
		":http",
		`The address to bind the http server to eg: ":http", "127.0.0.1:8080"`,
	)

	flag.BoolVar(
		&webInterface,
		"webinterface",
		true,
		"Runs PW2 with the web based password manager (as opposed to git only). "+
			"If PW2 is creating the repo for the first time, it will generate an associated GPG key for the "+
			"web interface user and mark it as admin in blackbox.",
	)
}

func getPassword(prompt string) (pw []byte, err error) {
	for len(pw) == 0 {
		fmt.Print(prompt)

		pw, err = terminal.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return
		}

		fmt.Print("\nRepeat that: ")

		var pwB []byte
		pwB, err = terminal.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return
		}

		fmt.Println()

		if !bytes.Equal(pw, pwB) {
			pw = nil
			glog.Info("Passwords did not match.")
			fmt.Println("Passwords did not match.")
		}
	}

	return
}

func do() (err error) {
	var db pw2.Database

	if _, err = os.Stat(repoDir); os.IsNotExist(err) {
		glog.Info("Database did not exist, creating...")

		var password []byte
		if webInterface {
			if password, err = getPassword("Web interface enabled, enter passphrase to use: "); err != nil {
				return
			}
		}

		glog.Info("Web interface enabled, prompting for GPG passphrase...")

		if db, err = pw2.Create(repoDir, password); err != nil {
			return
		}

		glog.Info("Creation completed")

		return

	}

	if db, err = pw2.Open(repoDir); err != nil {
		return
	}

	_ = db

	return
}

func main() {
	flag.Parse()

	if err := do(); err != nil {
		var buf bytes.Buffer
		switch v := err.(type) {
		default:
			_ = v
			fmt.Fprintf(&buf, "Fatal error: %+q %s", err, reflect.TypeOf(err))
		}
		glog.Fatal(buf.String())
	}

	return
}
