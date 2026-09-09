package memoryScanner

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// findProcess returns the PID of the first process matching name.
func findProcess(name string) (int, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(snap)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	if err := windows.Process32First(snap, &entry); err != nil {
		return 0, err
	}
	for {
		if windows.UTF16ToString(entry.ExeFile[:]) == name {
			return int(entry.ProcessID), nil
		}
		if err := windows.Process32Next(snap, &entry); err != nil {
			break
		}
	}
	return 0, fmt.Errorf("process %q not found", name)
}

// scanAuthz scans all readable memory regions of pid for the authz query string.
func scanAuthz(pid int) (string, error) {
	handle, err := windows.OpenProcess(
		windows.PROCESS_VM_READ|windows.PROCESS_QUERY_INFORMATION,
		false,
		uint32(pid),
	)
	if err != nil {
		return "", fmt.Errorf("failed to open process: %w", err)
	}
	defer windows.CloseHandle(handle)

	candidates := make(map[string]int)

	var addr uintptr
	var mbi windows.MemoryBasicInformation

	for {
		err := windows.VirtualQueryEx(handle, addr, &mbi, unsafe.Sizeof(mbi))
		if err != nil {
			break
		}

		// Only scan committed, readable regions.
		if mbi.State == windows.MEM_COMMIT &&
			mbi.Protect&windows.PAGE_NOACCESS == 0 &&
			mbi.Protect&windows.PAGE_GUARD == 0 {

			regionCandidates, err := scanRegionChunked(mbi.RegionSize, func(offset uintptr, dst []byte) (int, error) {
				var nRead uintptr
				err := windows.ReadProcessMemory(handle, addr+offset, &dst[0], uintptr(len(dst)), &nRead)
				return int(nRead), err
			})
			if err == nil {
				mergeCandidates(candidates, regionCandidates)
				if authz := findConfident(candidates); authz != "" {
					return authz, nil
				}
			}
		}

		addr += uintptr(mbi.RegionSize)
	}

	return "", fmt.Errorf("authz not found in process memory")
}
