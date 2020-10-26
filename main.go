package main

import (
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/alecthomas/kingpin.v3-unstable"
)

var (
	cli = kingpin.New("iw6x-zero", "Generates a package to run dedicated servers without unnecessary files.")

	sourceDir                string
	shouldSymlink            bool
	shouldFailOnSymlinkError bool
	targetDir                string
)

func init() {
	targetDir, _ = os.Getwd()

	cli.Flag("source", "The source game directory, specifically the directory where you have Call of Duty: Ghosts installed.").Short('s').Required().ExistingDirVar(&sourceDir)
	cli.Flag("symlink", "If set, this will make the tool symlink all files instead of copying them to save on disk space.").Short('l').BoolVar(&shouldSymlink)
	cli.Flag("symlink-fail", "If set, the tool will abort if an attempt at symlinking fials.").BoolVar(&shouldFailOnSymlinkError)
	cli.Arg("target", "The target directory to copy files to.").StringVar(&targetDir)
}

func main() {
	kingpin.MustParse(cli.Parse(os.Args[1:]))

	if len(targetDir) <= 0 {
		log.Fatal("No target directory was provided and the current working directory could not be read. You will need to define a target directory.")
	}

	sourceFs := newFileSystem(sourceDir)

	// do basic checks whether this is actually a Ghosts directory
	ok, mismatchedPath, allCheckedPaths, err := sourceFs.CheckGlobWhitelist(requiredFiles)
	if err != nil {
		log.Fatalf("An error occured while checking source directory: %s", err.Error())
	} else if !ok {
		log.Fatalf("The given source directory does not contain any file matching %s, so this is probably not a Call of Duty: Ghosts directory.", mismatchedPath)
	}

	var totalSize uint64 = 0

	targetFs := newFileSystem(targetDir)

	for _, relPath := range allCheckedPaths {
		dirPath := filepath.Dir(relPath)

		if shouldSymlink {
			// check if we can symlink the folder instead
			oldRelPath := relPath
			oldDirPath := dirPath
			for _, symlinkablePath := range symlinkableFolderPaths {
				if strings.HasPrefix(
					path.Clean(relPath),
					path.Clean(symlinkablePath)) {
					// link folder instead
					relPath = symlinkablePath
					dirPath = filepath.Dir(relPath)
					break
				}
			}

			targetInfo, err := targetFs.Lstat(relPath)
			if err != nil && !os.IsNotExist(err) {
				// something went wrong
				log.Fatal(err)
			}
			if err == nil && targetInfo.IsDir() {
				// do not attempt to create symlink if this is an existing directory,
				// work with the file path instead
				relPath = oldRelPath
				dirPath = oldDirPath
			}

			// do not attempt to create symlink if it already is a symlink pointing at the correct target
			if symlinkTarget, err := targetFs.Readlink(relPath); err == nil && symlinkTarget == sourceFs.FullPath(relPath) {
				continue
			}

			// create parent directories
			info, err := sourceFs.Stat(relPath)
			if err != nil {
				log.Fatal(err)
			}
			if err = targetFs.MkdirAll(dirPath, info.Mode()|0700); err != nil && !os.IsExist(err) {
				return
			}

			// symlink
			err = nil
			log.Println("Linking:", relPath)
			if err = sourceFs.SymlinkFromFs(relPath, targetFs); err == nil {
				info, err = targetFs.Stat(relPath)
				if err == nil {
					totalSize += uint64(info.Size())
				}
				continue
			} else if !shouldFailOnSymlinkError {
				log.Printf("Failed to create symlink, will now copy instead (reason was: %s)", err.Error())
				relPath = oldRelPath
				dirPath = oldDirPath
			} else {
				log.Fatalf("Failed to create symlink: %s", err.Error())
			}
		}

		// create parent directories
		info, err := sourceFs.Stat(relPath)
		if err != nil {
			log.Fatal(err)
		}
		if err = targetFs.MkdirAll(dirPath, info.Mode()|0700); err != nil && !os.IsExist(err) {
			return
		}

		// remove executable bit
		mode := info.Mode() & 0666

		// copy
		log.Println("Copying:", relPath)
		if err = sourceFs.CopyToFs(relPath, targetFs, mode); err != nil {
			log.Fatal(err)
		}

		totalSize += uint64(info.Size())
	}

	log.Println("All OK!")
}
