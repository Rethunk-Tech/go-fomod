// Package fomod reads FOMOD installers: it parses fomod/ModuleConfig.xml, evaluates its flag, file and game-version
// conditions, and turns a set of choices into the file and folder copy operations the installer declares. It copies
// nothing and reads no disk beyond the fs.FS it is handed.
package fomod
