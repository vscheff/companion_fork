package companion

import (
	"syscall"
	"unsafe"
)

func processNames() ([]string, error) {
	snap, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(snap)
	var pe syscall.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if err := syscall.Process32First(snap, &pe); err != nil {
		return nil, err
	}
	var out []string
	for {
		out = append(out, syscall.UTF16ToString(pe.ExeFile[:]))
		if err := syscall.Process32Next(snap, &pe); err != nil {
			break
		}
	}
	return out, nil
}
