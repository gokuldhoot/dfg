package main

import (
	"sync/atomic"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// File represents a file
type File struct {
	d    *Dir
	o    fs.Object
	size int64
}

// newFile creates a new File
func newFile(d *Dir, o fs.Object) *File {
	return &File{
		d: d,
		o: o,
	}
}

// Check interface satisfied
var _ fusefs.Node = (*File)(nil)

// Attr fills out the attributes for the file
func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	fs.Debug(f.o, "File.Attr")
	a.Mode = filePerms
	// if o is nil it isn't valid yet, so return the size so far
	if f.o == nil {
		a.Size = uint64(atomic.LoadInt64(&f.size))
	} else {
		a.Size = uint64(f.o.Size())
		if !*noModTime {
			modTime := f.o.ModTime()
			a.Atime = modTime
			a.Mtime = modTime
			a.Ctime = modTime
			a.Crtime = modTime
		}
	}
	return nil
}

// Update the size while writing
func (f *File) written(n int64) {
	atomic.AddInt64(&f.size, n)
}

// Update the object when written
func (f *File) setObject(o fs.Object) {
	f.o = o
	f.d.addObject(o, f)
}

// Check interface satisfied
var _ fusefs.NodeOpener = (*File)(nil)

// Open the file for read or write
func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fusefs.Handle, error) {
	fs.Debug(f.o, "File.Open")

	// Files aren't seekable
	resp.Flags |= fuse.OpenNonSeekable

	switch {
	case req.Flags.IsReadOnly():
		return newReadFileHandle(f.o)
	case req.Flags.IsWriteOnly():
		src := newCreateInfo(f.d.f, f.o.Remote())
		fh, err := newWriteFileHandle(f.d, f, src)
		if err != nil {
			return nil, err
		}
		return fh, nil
	case req.Flags.IsReadWrite():
		return nil, errors.New("can't open read and write")
	}

	/*
	   // File was opened in append-only mode, all writes will go to end
	   // of file. OS X does not provide this information.
	   OpenAppend    OpenFlags = syscall.O_APPEND
	   OpenCreate    OpenFlags = syscall.O_CREAT
	   OpenDirectory OpenFlags = syscall.O_DIRECTORY
	   OpenExclusive OpenFlags = syscall.O_EXCL
	   OpenNonblock  OpenFlags = syscall.O_NONBLOCK
	   OpenSync      OpenFlags = syscall.O_SYNC
	   OpenTruncate  OpenFlags = syscall.O_TRUNC
	*/
	return nil, errors.New("can't figure out how to open")
}
