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
	"reflect"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/golang/glog"
	"github.com/libgit2/git2go"
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

func do() (err error) {
	var db pw2.Database
	if db, err = pw2.Open(repoDir); err != nil {
		if pw2.DatabaseNotFound(err) {
			glog.Info("Database did not exist, creating...")
			var pw string
			if webInterface {
				for pw == "" {
					glog.Info("Web interface enabled, prompting for GPG passphrase...")

					fmt.Print("Web interface enabled, enter passphrase to use: ")

					var pwB []byte
					pwB, err = terminal.ReadPassword(int(syscall.Stdin))
					if err != nil {
						return
					}

					pw = string(pwB)

					fmt.Print("\nRepeat that: ")

					pwB, err = terminal.ReadPassword(int(syscall.Stdin))
					if err != nil {
						return
					}

					fmt.Println()

					if pw != string(pwB) {
						pw = ""
						glog.Info("Passwords did not match.")
						fmt.Println("Passwords did not match.")
					}
				}
			}

			db, err = pw2.Create(repoDir, pw)

			if err != nil {
				return
			}

			glog.Info("Creation completed")

		}
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
		case *git.GitError:
			var strCode, strClass string

			var ok bool
			if strCode, ok = gitErrorClassNames[v.Class]; !ok {
				strCode = fmt.Sprintf("#%v", v.Class)
			}

			if strClass, ok = gitErrorCodeNames[v.Code]; !ok {
				strClass = fmt.Sprintf("#%v", v.Code)
			}

			fmt.Fprintf(
				&buf,
				`Fatal git error:
	Code: %s
	Class: %s
	Message: %s`,
				strCode,
				strClass,
				v.Message,
			)

		default:
			fmt.Fprintf(&buf, "Fatal error: %+q %s", err, reflect.TypeOf(err))
		}
		glog.Fatal(buf.String())
	}

	return
}
