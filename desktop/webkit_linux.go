//go:build linux

package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	webkitSandboxEnv = "WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS"
	webkitDmabufEnv  = "WEBKIT_DISABLE_DMABUF_RENDERER"
)

var softwareDisplayDrivers = map[string]bool{
	"vmwgfx":             true,
	"virtio_gpu":         true,
	"virtio-gpu":         true,
	"qxl":                true,
	"vboxvideo":          true,
	"bochs":              true,
	"bochs-drm":          true,
	"cirrus":             true,
	"cirrus-qemu":        true,
	"hyperv_drm":         true,
	"simpledrm":          true,
	"simple-framebuffer": true,
	"efi-framebuffer":    true,
	"ast":                true,
	"mgag200":            true,
}

func configureWebKit() {
	if _, set := os.LookupEnv(webkitSandboxEnv); !set && !sandboxAvailable() {
		os.Setenv(webkitSandboxEnv, "1")
		log.Printf("bubblewrap cannot create a user namespace here; starting WebKit without its sandbox")
	}
	if _, set := os.LookupEnv(webkitDmabufEnv); !set && onlySoftwareDisplays() {
		os.Setenv(webkitDmabufEnv, "1")
		log.Printf("no hardware GPU found; turning off WebKit's DMA-BUF renderer")
	}
}

func sandboxAvailable() bool {
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bwrap, "--unshare-user", "--ro-bind", "/", "/", "--", "/bin/true")
	return cmd.Run() == nil
}

func onlySoftwareDisplays() bool {
	cards, _ := filepath.Glob("/sys/class/drm/card[0-9]*")
	found := false
	for _, card := range cards {
		target, err := filepath.EvalSymlinks(filepath.Join(card, "device", "driver"))
		if err != nil {
			continue
		}
		found = true
		if !softwareDisplayDrivers[filepath.Base(target)] {
			return false
		}
	}
	return found
}
