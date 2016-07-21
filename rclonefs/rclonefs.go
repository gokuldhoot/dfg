// FIXME rmdir
// FIXME infinite caching for directories?
// FIXME make a real Mkdir()?

package main

import (
	"fmt"
	"log"
	"os"

	"bazil.org/fuse"
	"github.com/ncw/rclone/fs"
	_ "github.com/ncw/rclone/fs/all"
	"github.com/spf13/pflag"
)

// Globals
var (
	// Flags
	version = pflag.BoolP("version", "V", false, "Print the version number")
	logFile = pflag.StringP("log-file", "", "", "Log everything to this file")
	// retries   = pflag.IntP("retries", "", 3, "Retry operations this many times if they fail")
	noModTime = pflag.BoolP("no-modtime", "", false, "Don't read the modification time (can speed things up).")
	debugFUSE = pflag.BoolP("debug-fuse", "", false, "Debug the FUSE internals - needs -v.")
)

// syntaxError prints the syntax
func syntaxError() {
	fmt.Fprintf(os.Stderr, `Mount rclone paths using FUSE - %s.

Syntax: [options] rclone:path/to/dir /path/to/mount/point

`, fs.Version)

	fmt.Fprintf(os.Stderr, "Options:\n")
	pflag.PrintDefaults()
}

// Exit with the message
func fatal(message string, args ...interface{}) {
	syntaxError()
	fmt.Fprintf(os.Stderr, message, args...)
	os.Exit(1)
}

func main() {
	pflag.Usage = syntaxError
	pflag.Parse()
	if *version {
		fmt.Printf("rclonefs %s\n", fs.Version)
		os.Exit(0)
	}

	if pflag.NArg() != 2 {
		fatal("2 arguments required\n")
	}

	// FIXME copied from rclone.go
	// Log file output
	if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
		if err != nil {
			log.Fatalf("Failed to open log file: %v", err)
		}
		_, err = f.Seek(0, os.SEEK_END)
		if err != nil {
			fs.ErrorLog(nil, "Failed to seek log file to end: %v", err)
		}
		log.SetOutput(f)
		fs.DebugLogger.SetOutput(f)
		// FIXME redirectStderr(f)
	}

	// Load the rest of the config now we have started the logger
	fs.LoadConfig()

	if *debugFUSE {
		fuse.Debug = func(msg interface{}) {
			fs.Debug("fuse", "%v", msg)
		}
	}

	path := pflag.Arg(0)
	mountpoint := pflag.Arg(1)

	// Make the remote
	f, err := fs.NewFs(path)
	if err != nil {
		log.Fatalf("Failed to make rclone remote: %v", err)
	}

	// Mount it
	errChan, err := mount(f, mountpoint)
	if err != nil {
		log.Fatalf("Failed to mount FUSE fs: %v", err)
	}

	// Wait for umount
	err = <-errChan
	if err != nil {
		log.Fatalf("Failed to umount FUSE fs: %v", err)
	}
}
