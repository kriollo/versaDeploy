package tui

import (
	"path/filepath"
	"strings"
)

// iconForEntry returns a colored Nerd Font (MDI v3) icon string for a file/directory entry.
// The returned string contains ANSI escapes — do not use it in fixed-width formatting.
func iconForEntry(name string, isDir bool) string {
	if isDir {
		return dirIcon(name)
	}
	return fileIcon(name)
}

// iconParentDir returns the icon for the ".." entry.
func iconParentDir() string {
	return iconColorFolder.Render("\U000F0770 ") // mdi-folder-open
}

func dirIcon(name string) string {
	lower := strings.ToLower(name)
	switch lower {
	case ".git", "git":
		return iconColorConfig.Render("\U000F02A2 ") // mdi-git
	case ".github":
		return iconColorConfig.Render("\U000F02A4 ") // mdi-github
	case "node_modules":
		return iconColorJS.Render("\U000F031E ") // mdi-language-javascript
	case "vendor":
		return iconColorFolder.Render("\U000F024B ") // mdi-folder
	case "dist", "build", "out", "output":
		return iconColorFolder.Render("\U000F024B ") // mdi-folder
	case "src", "source":
		return iconColorFolder.Render("\U000F024B ") // mdi-folder
	case "releases":
		return iconColorFolder.Render("\U000F024B ") // mdi-folder
	case "shared", "share":
		return iconColorFolder.Render("\U000F024B ") // mdi-folder
	case "logs", "log":
		return iconColorConfig.Render("\U000F024B ") // mdi-folder
	case "config", "configs", "conf", "etc":
		return iconColorConfig.Render("\U000F0493 ") // mdi-cog
	case "test", "tests", "__tests__":
		return iconColorFolder.Render("\U000F024B ") // mdi-folder
	case "docs", "documentation":
		return iconColorFolder.Render("\U000F024B ") // mdi-folder
	case "assets", "static", "public":
		return iconColorFolder.Render("\U000F024B ") // mdi-folder
	case "scripts", "bin":
		return iconColorConfig.Render("\U000F0239 ") // mdi-console
	case "migrations":
		return iconColorConfig.Render("\U000F01BC ") // mdi-database
	case ".vscode":
		return iconColorConfig.Render("\U000F0493 ") // mdi-cog
	case "tmp", "temp", "cache":
		return iconColorMuted.Render("\U000F024B ") // mdi-folder
	case "deploy", "deployments":
		return iconColorFolder.Render("\U000F024B ") // mdi-folder
	case "api":
		return iconColorFolder.Render("\U000F024B ") // mdi-folder
	default:
		return iconColorFolder.Render("\U000F024B ") // mdi-folder
	}
}

// iconColorMuted is a convenience alias used in dirIcon for low-importance folders
var iconColorMuted = iconColorConfig

func fileIcon(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	baseLower := strings.ToLower(name)

	// Exact filename matches first
	switch baseLower {
	case "dockerfile", "containerfile":
		return iconColorTS.Render("\U000F0868 ") // mdi-docker
	case "makefile", "gnumakefile":
		return iconColorConfig.Render("\U000F0493 ") // mdi-cog
	case "readme.md", "readme.txt", "readme":
		return iconColorFolder.Render("\U000F0354 ") // mdi-language-markdown
	case ".gitignore", ".gitattributes", ".gitmodules":
		return iconColorConfig.Render("\U000F02A2 ") // mdi-git
	case ".env", ".env.local", ".env.example":
		return iconColorLock.Render("\U000F033E ") // mdi-lock
	case "go.mod", "go.sum":
		return iconColorGo.Render("\U000F07D3 ") // mdi-language-go
	case "package.json", "package-lock.json", "yarn.lock":
		return iconColorJS.Render("\U000F031E ") // mdi-language-javascript
	case "composer.json", "composer.lock":
		return iconColorPHP.Render("\U000F031F ") // mdi-language-php
	case "requirements.txt", "pipfile", "pipfile.lock", "pyproject.toml", "setup.py", "setup.cfg":
		return iconColorPython.Render("\U000F0320 ") // mdi-language-python
	case "cargo.toml", "cargo.lock":
		return iconColorRust.Render("\U000F1617 ") // mdi-language-rust
	case "gemfile", "gemfile.lock":
		return iconColorRuby.Render("\U000F0214 ") // mdi-file (ruby icon)
	case "deploy.yml", "deploy.yaml":
		return iconColorFolder.Render("\U000F0493 ") // mdi-cog
	case "deploy.lock":
		return iconColorLock.Render("\U000F033E ") // mdi-lock
	}

	// Extension-based icons
	switch ext {
	// Go
	case ".go":
		return iconColorGo.Render("\U000F07D3 ") // mdi-language-go
	// Python
	case ".py", ".pyw", ".pyx", ".pxd":
		return iconColorPython.Render("\U000F0320 ") // mdi-language-python
	// JavaScript / TypeScript
	case ".js", ".mjs", ".cjs":
		return iconColorJS.Render("\U000F031E ") // mdi-language-javascript
	case ".ts":
		return iconColorTS.Render("\U000F06E6 ") // mdi-language-typescript
	case ".jsx":
		return iconColorJS.Render("\U000F031E ") // mdi-language-javascript
	case ".tsx":
		return iconColorTS.Render("\U000F06E6 ") // mdi-language-typescript
	case ".vue":
		return iconColorSuccess.Render("\U000F0214 ") // mdi-file (vue)
	case ".svelte":
		return iconColorRust.Render("\U000F0214 ") // mdi-file (svelte)
	// PHP
	case ".php":
		return iconColorPHP.Render("\U000F031F ") // mdi-language-php
	// Rust
	case ".rs":
		return iconColorRust.Render("\U000F1617 ") // mdi-language-rust
	// Ruby
	case ".rb", ".erb":
		return iconColorRuby.Render("\U000F0214 ") // mdi-file
	// Java / JVM
	case ".java", ".class", ".jar":
		return iconColorWarning.Render("\U000F0214 ") // mdi-file
	case ".kt", ".kts":
		return iconColorTS.Render("\U000F0214 ") // mdi-file
	case ".scala":
		return iconColorError.Render("\U000F0214 ") // mdi-file
	case ".groovy":
		return iconColorConfig.Render("\U000F0214 ") // mdi-file
	// C family
	case ".c", ".h":
		return iconColorTS.Render("\U000F0214 ") // mdi-file
	case ".cpp", ".cc", ".cxx", ".hpp", ".hxx":
		return iconColorTS.Render("\U000F0214 ") // mdi-file
	case ".cs":
		return iconColorTS.Render("\U000F0214 ") // mdi-file
	// Shell scripts
	case ".sh", ".bash", ".zsh", ".fish", ".ksh", ".csh":
		return iconColorConfig.Render("\U000F0239 ") // mdi-console
	// Web
	case ".html", ".htm":
		return iconColorRust.Render("\U000F031B ") // mdi-language-html5
	case ".css":
		return iconColorTS.Render("\U000F031C ") // mdi-language-css3
	case ".scss", ".sass", ".less":
		return iconColorPHP.Render("\U000F031C ") // mdi-language-css3
	// Data / Config
	case ".json":
		return iconColorWarning.Render("\U000F0214 ") // mdi-file
	case ".yml", ".yaml":
		return iconColorConfig.Render("\U000F0493 ") // mdi-cog
	case ".toml":
		return iconColorConfig.Render("\U000F0493 ") // mdi-cog
	case ".xml":
		return iconColorConfig.Render("\U000F0214 ") // mdi-file
	case ".ini", ".conf", ".cfg", ".config":
		return iconColorConfig.Render("\U000F0493 ") // mdi-cog
	case ".env":
		return iconColorLock.Render("\U000F033E ") // mdi-lock
	// Docs
	case ".md", ".markdown":
		return iconColorFolder.Render("\U000F0354 ") // mdi-language-markdown
	case ".txt":
		return iconColorConfig.Render("\U000F0214 ") // mdi-file
	case ".pdf":
		return iconColorError.Render("\U000F0214 ") // mdi-file
	case ".doc", ".docx":
		return iconColorTS.Render("\U000F0214 ") // mdi-file
	case ".xls", ".xlsx":
		return iconColorSuccess.Render("\U000F0214 ") // mdi-file
	case ".ppt", ".pptx":
		return iconColorRust.Render("\U000F0214 ") // mdi-file
	// Images
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".tiff", ".ico":
		return iconColorImage.Render("\U000F0232 ") // mdi-image
	case ".svg":
		return iconColorImage.Render("\U000F0232 ") // mdi-image
	// Archives
	case ".zip", ".tar", ".gz", ".tgz", ".bz2", ".xz", ".7z", ".rar":
		return iconColorArchive.Render("\U000F05C4 ") // mdi-archive
	// Binary / executables
	case ".exe", ".dll", ".so", ".dylib":
		return iconColorConfig.Render("\U000F0493 ") // mdi-cog
	// Logs
	case ".log":
		return iconColorConfig.Render("\U000F0214 ") // mdi-file
	// SQL / DB
	case ".sql":
		return iconColorConfig.Render("\U000F01BC ") // mdi-database
	case ".db", ".sqlite", ".sqlite3":
		return iconColorConfig.Render("\U000F01BC ") // mdi-database
	// Docker
	case ".dockerignore":
		return iconColorTS.Render("\U000F0868 ") // mdi-docker
	// Lock files
	case ".lock":
		return iconColorLock.Render("\U000F033E ") // mdi-lock
	// Certificate / keys
	case ".pem", ".crt", ".cer", ".key", ".pub":
		return iconColorLock.Render("\U000F033E ") // mdi-lock
	// Video
	case ".mp4", ".mkv", ".avi", ".mov", ".webm":
		return iconColorConfig.Render("\U000F0214 ") // mdi-file
	// Audio
	case ".mp3", ".wav", ".flac", ".ogg", ".m4a":
		return iconColorConfig.Render("\U000F0214 ") // mdi-file
	default:
		return iconColorConfig.Render("\U000F0214 ") // mdi-file
	}
}

// iconColorSuccess and iconColorWarning and iconColorError are convenience aliases
// for use inside icons.go that map to the existing style palette.
var (
	iconColorSuccess = iconColorImage   // green
	iconColorWarning = iconColorLock    // yellow/warning
	iconColorError   = iconColorArchive // red
)
