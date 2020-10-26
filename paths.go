package main

var (
	requiredFiles = []string{
		"main/fileSysCheck.cfg",
		"zone/**.ff",
		"iw6mp64_ship.exe",
	}

	symlinkableFolderPaths = []string{
		"APEX",
		"main",
		"zone",
	}

	windowsFiles = []string{
		"advapi32.dll",
		"d3d11.dll",
		"dxgi.dll",
		"gdi32.dll",
		"kernel32.dll",
		"ole32.dll",
		"powrprof.dll",
		"psapi.dll",
		"shell32.dll",
		"user32.dll",
		"winmm.dll",
	}
)
