//go:build darwin

package main

// Wails v2's own macOS webview glue references UTType (from the
// UniformTypeIdentifiers framework, used for drag-and-drop file type
// checks) but its cgo directives never link that framework explicitly.
// Recent Xcode/SDK combinations no longer pull it in transitively, so the
// final link fails with "Undefined symbols ... _OBJC_CLASS_$_UTType" for
// *any* macOS build of this app — unrelated to Karina's own code. This file
// only exists to add the missing link flag.

/*
#cgo LDFLAGS: -framework UniformTypeIdentifiers
*/
import "C"
