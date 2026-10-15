package remote

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"
)

func safeFilename(name, system string) string {
	name = strings.Map(func(r rune) rune {
		if r == 0 || r == '/' || system == "windows" && (r < 32 || strings.ContainsRune(`<>:"\|?*`, r)) {
			return '_'
		}
		return r
	}, name)
	if system == "windows" {
		name = strings.TrimRight(name, " .")
		base := strings.ToUpper(strings.TrimRight(strings.SplitN(name, ".", 2)[0], " "))
		reserved := base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || base == "CONIN$" || base == "CONOUT$"
		baseRunes := []rune(base)
		if len(baseRunes) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && strings.ContainsRune("123456789¹²³", baseRunes[3]) {
			reserved = true
		}
		if reserved {
			name = "_" + name
		}
	}
	if name == "" || name == "." || name == ".." {
		name = "_log"
	}
	if filenameUnits(name, system) > 255 {
		ext := filepath.Ext(name)
		if filenameUnits(ext, system) >= 255 {
			ext = ""
		}
		name = truncateName(strings.TrimSuffix(name, ext), 255-filenameUnits(ext, system), system) + ext
	}
	return name
}

func filenameUnits(name, system string) int {
	if system != "windows" {
		return len(name)
	}
	n := 0
	for _, r := range name {
		n++
		if r > 0xffff {
			n++
		}
	}
	return n
}

func truncateName(name string, limit int, system string) string {
	for filenameUnits(name, system) > limit && name != "" {
		_, width := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-width]
	}
	return name
}

func downloadFilename(directory, name string) (string, error) {
	name = safeFilename(name, runtime.GOOS)
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for n := 0; ; n++ {
		candidate := name
		if n > 0 {
			suffix := fmt.Sprintf(" (%d)", n)
			extension := truncateName(ext, 254-filenameUnits(suffix, runtime.GOOS), runtime.GOOS)
			prefix := truncateName(stem, 255-filenameUnits(suffix+extension, runtime.GOOS), runtime.GOOS)
			candidate = prefix + suffix + extension
		}
		if strings.EqualFold(candidate, "manifest.json") {
			continue
		}
		target := filepath.Join(directory, candidate)
		if _, err := os.Lstat(target); os.IsNotExist(err) {
			return target, nil
		} else if err != nil {
			return "", err
		}
	}
}
