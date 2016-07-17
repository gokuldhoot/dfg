package main

import (
	"os"
	"path"
	"sync"
	"time"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// Dir represents a directory entry
type Dir struct {
	f     fs.Fs
	path  string
	mu    sync.RWMutex // protects the following
	read  bool
	items map[string]fs.BasicInfo
}

func newDir(f fs.Fs, path string) *Dir {
	return &Dir{
		f:    f,
		path: path,
	}
}

// addObject adds a new object or directory to the directory
func (d *Dir) addObject(o fs.BasicInfo) {
	d.mu.Lock()
	d.items[path.Base(o.Remote())] = o
	d.mu.Unlock()
}

// delObject removes an object from the directory
func (d *Dir) delObject(leaf string) {
	d.mu.Lock()
	delete(d.items, leaf)
	d.mu.Unlock()
}

// read the directory
func (d *Dir) readDir() error {
	fs.Debug(d.f, "Dir.readDir(%q)", d.path)
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.read {
		return nil
	}
	objs, dirs, err := fs.NewLister().SetLevel(1).Start(d.f, d.path).GetAll()
	if err == fs.ErrorDirNotFound {
		// We treat directory not found as empty because we
		// create directories on the fly
	} else if err != nil {
		return err
	}
	// Cache the items by name
	d.items = make(map[string]fs.BasicInfo, len(objs)+len(dirs))
	for _, obj := range objs {
		name := path.Base(obj.Remote())
		d.items[name] = obj
	}
	for _, dir := range dirs {
		name := path.Base(dir.Remote())
		d.items[name] = dir
	}
	d.read = true
	return nil
}

// lookup a single item in the directory
//
// returns fuse.ENOENT if not found.
func (d *Dir) lookup(leaf string) (fs.BasicInfo, error) {
	err := d.readDir()
	if err != nil {
		return nil, err
	}
	d.mu.RLock()
	item, ok := d.items[leaf]
	d.mu.RUnlock()
	if !ok {
		return nil, fuse.ENOENT
	}
	return item, nil
}

// Check interface satsified
var _ fusefs.Node = (*Dir)(nil)

// Attr updates the attribes of a directory
func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	fs.Debug(d.f, "Dir.Attr(%q)", d.path)
	a.Mode = os.ModeDir | dirPerms
	return nil
}

// Check interface satisfied
var _ fusefs.NodeRequestLookuper = (*Dir)(nil)

// Lookup looks up a specific entry in the receiver.
//
// Lookup should return a Node corresponding to the entry.  If the
// name does not exist in the directory, Lookup should return ENOENT.
//
// Lookup need not to handle the names "." and "..".
func (d *Dir) Lookup(ctx context.Context, req *fuse.LookupRequest, resp *fuse.LookupResponse) (fusefs.Node, error) {
	path := path.Join(d.path, req.Name)
	fs.Debug(d.f, "Dir.Lookup(%q)", path)
	item, err := d.lookup(req.Name)
	if err != nil {
		return nil, err
	}
	switch x := item.(type) {
	case fs.Object:
		return newFile(d, x), nil
	case *fs.Dir:
		return newDir(d.f, x.Remote()), nil
	default:
		return nil, errors.Errorf("unknown type %T", item)
	}
}

// Check interface satisfied
var _ fusefs.HandleReadDirAller = (*Dir)(nil)

// ReadDirAll reads the contents of the directory
func (d *Dir) ReadDirAll(ctx context.Context) (dirents []fuse.Dirent, err error) {
	fs.Debug(d.f, "Dir.ReadDirAll(%q)", d.path)
	err = d.readDir()
	if err != nil {
		return nil, err
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, item := range d.items {
		var dirent fuse.Dirent
		switch x := item.(type) {
		case fs.Object:
			dirent = fuse.Dirent{
				// Inode FIXME ???
				Type: fuse.DT_File,
				Name: path.Base(x.Remote()),
			}
		case *fs.Dir:
			dirent = fuse.Dirent{
				// Inode FIXME ???
				Type: fuse.DT_Dir,
				Name: path.Base(x.Remote()),
			}
		default:
			return nil, errors.Errorf("unknown type %T", item)
		}
		dirents = append(dirents, dirent)
	}
	return dirents, nil
}

var _ fusefs.NodeCreater = (*Dir)(nil)

// Create makes a new file
func (d *Dir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fusefs.Node, fusefs.Handle, error) {
	path := path.Join(d.path, req.Name)
	fs.Debug(d.f, "Dir.Create(%q)", path)
	src := newCreateInfo(d.f, path)
	file := newFile(d, nil)
	fh, err := newWriteFileHandle(d, file, src)
	if err != nil {
		return nil, nil, err
	}
	return file, fh, nil
}

var _ fusefs.NodeMkdirer = (*Dir)(nil)

// Mkdir creates a new directory
func (d *Dir) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fusefs.Node, error) {
	// We just pretend to have created the directory - rclone will
	// actually create the directory if we write files into it
	path := path.Join(d.path, req.Name)
	fs.Debug(d.f, "Dir.Mkdir(%q)", path)
	fsDir := &fs.Dir{
		Name: path,
		When: time.Now(),
	}
	d.addObject(fsDir)
	dir := newDir(d.f, path)
	return dir, nil
}

// dirEmpty returns if the directory path passed in is empty or not
func (d *Dir) dirEmpty(path string) (empty bool, err error) {
	lister := fs.NewLister().SetLevel(1).Start(d.f, path)
	defer lister.Finished()
	obj, dir, err := lister.Get()
	switch {
	case err != nil:
		if err == fs.ErrorDirNotFound {
			err = nil
		}
		empty = true
	case obj != nil:
		empty = false
	case dir != nil:
		empty = false
	default:
		empty = true
	}
	return empty, err
}

var _ fusefs.NodeRemover = (*Dir)(nil)

// Remove removes the entry with the given name from
// the receiver, which must be a directory.  The entry to be removed
// may correspond to a file (unlink) or to a directory (rmdir).
func (d *Dir) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	path := path.Join(d.path, req.Name)
	fs.Debug(d.f, "Dir.Remove(%q)", path)
	item, err := d.lookup(req.Name)
	if err != nil {
		return err
	}
	switch x := item.(type) {
	case fs.Object:
		err = x.Remove()
		if err != nil {
			return err
		}
	case *fs.Dir:
		// Do nothing for deleting directory - rclone can't
		// currently remote a random directory
		//
		// Check directory is empty
		empty, err := d.dirEmpty(path)
		if err != nil {
			return err
		}
		if !empty {
			// return fuse.ENOTEMPTY - doesn't exist though so use EEXIST
			return fuse.EEXIST
		}
	default:
		return errors.Errorf("unknown type %T", item)
	}
	// Remove the item from the directory listing
	d.delObject(req.Name)
	return nil
}
