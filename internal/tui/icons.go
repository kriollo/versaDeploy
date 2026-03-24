package tui

import (
	"path/filepath"
	"strings"
)

// iconForEntry returns a Nerd Font icon string for a file/directory entry.
// Uses Font Awesome and Devicon codepoints from Nerd Fonts v3, which render
// reliably in most Nerd Font terminal configurations.
func iconForEntry(name string, isDir bool) string {
	if isDir {
		return dirIcon(name)
	}
	return fileIcon(name)
}

// iconParentDir returns the icon for the ".." entry.
func iconParentDir() string {
	return "\uf07c " // nf-fa-folder_open
}

func dirIcon(name string) string {
	lower := strings.ToLower(name)
	switch lower {
	case ".git", "git":
		return "\ue702 " // nf-dev-git_branch
	case "node_modules":
		return "\ue718 " // nf-dev-nodejs_small
	case "vendor":
		return "\uf07b " // nf-fa-folder
	case "dist", "build", "out", "output":
		return "\uf466 " // nf-oct-package
	case "src", "source":
		return "\uf07b " // nf-fa-folder
	case ".github":
		return "\uf09b " // nf-fa-github
	case "releases":
		return "\uf02c " // nf-fa-tags
	case "shared", "share":
		return "\uf0c1 " // nf-fa-link
	case "logs", "log":
		return "\uf15c " // nf-fa-file_text
	case "config", "configs", "conf", "etc":
		return "\uf013 " // nf-fa-gear
	default:
		return "\uf07b " // nf-fa-folder
	}
}

func fileIcon(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	baseLower := strings.ToLower(name)

	// Exact filename matches first
	switch baseLower {
	case "dockerfile", "containerfile":
		return "\ue7b0 " // nf-dev-docker
	case "makefile", "gnumakefile":
		return "\uf0ad " // nf-fa-wrench
	case "readme.md", "readme.txt", "readme":
		return "\uf02d " // nf-fa-book
	case ".gitignore", ".gitattributes", ".gitmodules":
		return "\ue702 " // nf-dev-git_branch
	case ".env", ".env.local", ".env.example":
		return "\uf023 " // nf-fa-lock
	case "go.mod", "go.sum":
		return "\ue627 " // nf-seti-go
	case "package.json", "package-lock.json", "yarn.lock":
		return "\ue718 " // nf-dev-npm
	case "composer.json", "composer.lock":
		return "\ue73d " // nf-dev-php
	case "requirements.txt", "pipfile", "pipfile.lock", "pyproject.toml", "setup.py", "setup.cfg":
		return "\ue73c " // nf-dev-python
	case "cargo.toml", "cargo.lock":
		return "\ue7a8 " // nf-dev-rust
	case "gemfile", "gemfile.lock":
		return "\ue739 " // nf-dev-ruby
	case "deploy.yml", "deploy.yaml":
		return "\uf0c5 " // nf-fa-file_o
	case "deploy.lock":
		return "\uf023 " // nf-fa-lock
	}

	// Extension-based icons
	switch ext {
	// Go
	case ".go":
		return "\ue627 " // nf-seti-go
	// Python
	case ".py", ".pyw", ".pyx", ".pxd":
		return "\ue73c " // nf-dev-python
	// JavaScript / TypeScript
	case ".js", ".mjs", ".cjs":
		return "\ue74e " // nf-dev-javascript_badge
	case ".ts":
		return "\ue628 " // nf-seti-typescript
	case ".jsx":
		return "\ue7ba " // nf-dev-react
	case ".tsx":
		return "\ue7ba " // nf-dev-react
	case ".vue":
		return "\ue76e " // nf-dev-vue (FA range)
	case ".svelte":
		return "\ue697 " // nf-dev-svelte
	// PHP
	case ".php":
		return "\ue73d " // nf-dev-php
	// Rust
	case ".rs":
		return "\ue7a8 " // nf-dev-rust
	// Ruby
	case ".rb", ".erb":
		return "\ue739 " // nf-dev-ruby
	// Java / JVM
	case ".java", ".class", ".jar":
		return "\ue738 " // nf-dev-java
	case ".kt", ".kts":
		return "\ue634 " // nf-seti-kotlin
	case ".scala":
		return "\ue737 " // nf-dev-scala
	case ".groovy":
		return "\uf30b " // nf-mdi-language_java (approx)
	// C family
	case ".c", ".h":
		return "\ue61e " // nf-custom-c
	case ".cpp", ".cc", ".cxx", ".hpp", ".hxx":
		return "\ue61d " // nf-custom-cpp
	case ".cs":
		return "\ue648 " // nf-seti-csharp
	// Shell scripts
	case ".sh", ".bash", ".zsh", ".fish", ".ksh", ".csh":
		return "\uf120 " // nf-fa-terminal
	// Web
	case ".html", ".htm":
		return "\ue736 " // nf-dev-html5
	case ".css":
		return "\ue749 " // nf-dev-css3
	case ".scss", ".sass", ".less":
		return "\ue603 " // nf-dev-sass
	// Data / Config
	case ".json":
		return "\ue60b " // nf-seti-json
	case ".yml", ".yaml":
		return "\ue60b " // nf-seti-json (reuse)
	case ".toml":
		return "\ue60b "
	case ".xml":
		return "\uf72d " // nf-mdi-xml
	case ".ini", ".conf", ".cfg", ".config":
		return "\uf013 " // nf-fa-gear
	case ".env":
		return "\uf023 " // nf-fa-lock
	// Docs
	case ".md", ".markdown":
		return "\uf48a " // nf-oct-markdown
	case ".txt":
		return "\uf15b " // nf-fa-file
	case ".pdf":
		return "\uf1c1 " // nf-fa-file_pdf_o
	case ".doc", ".docx":
		return "\uf1c2 " // nf-fa-file_word_o
	case ".xls", ".xlsx":
		return "\uf1c3 " // nf-fa-file_excel_o
	case ".ppt", ".pptx":
		return "\uf1c4 " // nf-fa-file_powerpoint_o
	// Images
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".tiff", ".ico":
		return "\uf03e " // nf-fa-image
	case ".svg":
		return "\uf03e " // nf-fa-image
	// Archives
	case ".zip", ".tar", ".gz", ".tgz", ".bz2", ".xz", ".7z", ".rar":
		return "\uf1c6 " // nf-fa-file_archive_o
	// Binary / executables
	case ".exe", ".dll", ".so", ".dylib":
		return "\uf013 " // nf-fa-gear
	// Logs
	case ".log":
		return "\uf15c " // nf-fa-file_text
	// SQL / DB
	case ".sql":
		return "\uf1c0 " // nf-fa-database
	case ".db", ".sqlite", ".sqlite3":
		return "\uf1c0 "
	// Docker
	case ".dockerignore":
		return "\ue7b0 " // nf-dev-docker
	// Lock files
	case ".lock":
		return "\uf023 " // nf-fa-lock
	// Certificate / keys
	case ".pem", ".crt", ".cer", ".key", ".pub":
		return "\uf023 " // nf-fa-lock
	// Video
	case ".mp4", ".mkv", ".avi", ".mov", ".webm":
		return "\uf1c8 " // nf-fa-file_video_o
	// Audio
	case ".mp3", ".wav", ".flac", ".ogg", ".m4a":
		return "\uf1c7 " // nf-fa-file_audio_o
	default:
		return "\uf15b " // nf-fa-file
	}
}
