package tui

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// guiPickDir — нативный диалог выбора папки.
func guiPickDir(title string) (string, bool) {
	return guiPick(title, true)
}

// guiPickArchive — диалог выбора архива (.tar.gz/.zip) для unpack.
func guiPickArchive(title string) (string, bool) {
	return guiPick(title, false)
}

// guiPickFile — диалог выбора файла с kdialog-фильтром вида "*.yaml *.yml|YAML configs".
func guiPickFile(title, filter string) (string, bool) {
	if os.Getenv("QPACK_NO_GUI") != "" {
		return "", false
	}
	if runtime.GOOS == "windows" {
		return windowsPickFile(title, filter)
	}
	if dir, ok := runDlg("kdialog", "--getopenfilename", ".", filter, "--title", title); ok {
		return dir, true
	}
	zenFilter := "--file-filter=" + strings.Split(filter, "|")[0]
	if dir, ok := runDlg("zenity", "--file-selection", "--title="+title, zenFilter); ok {
		return dir, true
	}
	return "", false
}

func guiPick(title string, dirOnly bool) (string, bool) {
	if os.Getenv("QPACK_NO_GUI") != "" {
		return "", false
	}
	switch runtime.GOOS {
	case "windows":
		return windowsPick(title, dirOnly)
	default:
		if dirOnly {
			if dir, ok := runDlg("kdialog", "--getexistingdirectory", ".", "--title", title); ok {
				return dir, true
			}
			if dir, ok := runDlg("zenity", "--file-selection", "--directory", "--title="+title); ok {
				return dir, true
			}
			return "", false
		}
		if dir, ok := runDlg("kdialog", "--getopenfilename", ".", "*.tar.gz *.tgz *.zip|Archives", "--title", title); ok {
			return dir, true
		}
		if dir, ok := runDlg("zenity", "--file-selection", "--title="+title, "--file-filter=*.tar.gz *.tgz *.zip"); ok {
			return dir, true
		}
		return "", false
	}
}

func runDlg(prog string, args ...string) (string, bool) {
	path, err := exec.LookPath(prog)
	if err != nil {
		return "", false
	}
	out, err := exec.Command(path, args...).Output()
	if err != nil {
		return "", false
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		return "", false
	}
	return dir, true
}

func windowsPick(title string, dirOnly bool) (string, bool) {
	path, err := exec.LookPath("powershell")
	if err != nil {
		return "", false
	}
	safe := strings.ReplaceAll(title, `'`, `''`)
	var script string
	if dirOnly {
		script = `Add-Type -AssemblyName System.Windows.Forms; ` +
			`$d = New-Object System.Windows.Forms.FolderBrowserDialog; ` +
			`$d.Description = '` + safe + `'; ` +
			`$d.ShowNewFolderButton = $true; ` +
			`if ($d.ShowDialog() -eq 'OK') { $d.SelectedPath }`
	} else {
		script = `Add-Type -AssemblyName System.Windows.Forms; ` +
			`$d = New-Object System.Windows.Forms.OpenFileDialog; ` +
			`$d.Title = '` + safe + `'; ` +
			`$d.Filter = 'Archives (*.tar.gz;*.tgz;*.zip)|*.tar.gz;*.tgz;*.zip|All files (*.*)|*.*'; ` +
			`if ($d.ShowDialog() -eq 'OK') { $d.FileName }`
	}
	out, err := exec.Command(path, "-NoProfile", "-Command", script).Output()
	if err != nil {
		return "", false
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		return "", false
	}
	return dir, true
}
func windowsPickFile(title, filter string) (string, bool) {
	path, err := exec.LookPath("powershell")
	if err != nil {
		return "", false
	}
	safe := strings.ReplaceAll(title, `'`, `''`)
	desc := strings.ReplaceAll(strings.Split(filter, "|")[0], `;`, ` `)
	script := `Add-Type -AssemblyName System.Windows.Forms; ` +
		`$d = New-Object System.Windows.Forms.OpenFileDialog; ` +
		`$d.Title = '` + safe + `'; ` +
		`$d.Filter = '` + desc + `|*.*|All files (*.*)|*.*'; ` +
		`if ($d.ShowDialog() -eq 'OK') { $d.FileName }`
	out, err := exec.Command(path, "-NoProfile", "-Command", script).Output()
	if err != nil {
		return "", false
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		return "", false
	}
	return dir, true
}
