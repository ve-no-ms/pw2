/*
Pw2 is a password management utility akin to 1password, lastpass and keepass.
Instead of having your passwords stored on a remote server you don't own encrypted
with dubious cryptography. Pw2 keeps your passwords on a server you do own,
and encrypts all your password data with GPG2.

Pw2 has a number of features beyond other password managers, mostly as side-effects
to its construction. Pw2 is built with the Go HTTP server, git and blackbox
by StackOverflow.

Blackbox is a technology that uses GPG to store secrets in git repositories.
Blackbox allows several users to be authorized to encrypt and decrypt secrets.
Using git to store passwords has a number of benefits such as logging,
revision management and conflict resolution.

GPG is also great because you can use your pre-existing keys, subkeys, smarcards,
Yubikeys and all that wonderful stuff.

In the past, I used Keepass to manage my passwords, and either tunneled access
through SSH or copied the database to multiple computers. This caused problems
when I made changes on two computers, I'd need to edit the password database
on one or the other of them and manually resolve the conflict.

Another issue of Keepass is that you have a database that needs a desktop utility
to read. I'm hoping to expose a usable HTTP interface for use on mobile devices and
anything else that supports the web.

Using this software

Pw2 can be used in two different ways: the first is importing the subpackage pkg/pw2
and using its http.Handler, binding it to some route on your existing Go server,
the second is running cmd/pw2, which runs a Go http server with some configurable flags.
*/
package pw2
