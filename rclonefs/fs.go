// FUSE main Fs

package main

import (
	"log"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/ncw/rclone/fs"
)

// Default permissions
const (
	dirPerms  = 0755
	filePerms = 0644
)

// FS represents the top level filing system
type FS struct {
	f fs.Fs
}

// Check interface satistfied
var _ fusefs.FS = (*FS)(nil)

// Root returns the root node
func (f *FS) Root() (fusefs.Node, error) {
	fs.Debug(f.f, "Root()")
	return newDir(f.f, ""), nil
}

// mount the file system - doesn't return until the mount is finished with
func mount(remote, mountpoint string) error {
	f, err := fs.NewFs(remote)
	if err != nil {
		return err
	}

	c, err := fuse.Mount(mountpoint)
	if err != nil {
		return err
	}
	defer func() {
		err := c.Close()
		if err != nil {
			log.Printf("Close returned error: %v", err)
		}
	}()

	filesys := &FS{
		f: f,
	}
	if err := fusefs.Serve(c, filesys); err != nil {
		return err
	}

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		return err
	}

	return nil
}
