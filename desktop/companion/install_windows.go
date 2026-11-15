package companion

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var procQueryFullProcessImageName = syscall.NewLazyDLL("kernel32.dll").NewProc("QueryFullProcessImageNameW")

const (
	processTerminate               = 0x0001
	processQueryLimitedInformation = 0x1000
	synchronize                    = 0x00100000
)

func processImagePath(h syscall.Handle) string {
	buf := make([]uint16, syscall.MAX_LONG_PATH)
	n := uint32(len(buf))
	r, _, _ := procQueryFullProcessImageName.Call(uintptr(h), 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}

func stopInstalledCopy(target string) {
	snap, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return
	}
	defer syscall.CloseHandle(snap)
	var pe syscall.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	self := uint32(os.Getpid())
	base := filepath.Base(target)
	for err = syscall.Process32First(snap, &pe); err == nil; err = syscall.Process32Next(snap, &pe) {
		if pe.ProcessID == self || !strings.EqualFold(syscall.UTF16ToString(pe.ExeFile[:]), base) {
			continue
		}
		h, err := syscall.OpenProcess(processTerminate|processQueryLimitedInformation|synchronize, false, pe.ProcessID)
		if err != nil {
			continue
		}
		if sameInstallPath(processImagePath(h), target) && syscall.TerminateProcess(h, 0) == nil {
			_, _ = syscall.WaitForSingleObject(h, 5000)
		}
		syscall.CloseHandle(h)
	}
}
