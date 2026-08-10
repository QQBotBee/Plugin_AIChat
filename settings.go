//go:build windows

package main

import (
	"context"
	"os"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	settingsBaseWidth   = 560
	settingsBaseHeight  = 260
	settingsBasePadding = 18
)

type settingsLayout struct {
	ClientWidth  int
	ClientHeight int
	Padding      int
	Resizable    bool
	Maximizable  bool
}

func scaleDPI(value, dpi int) int {
	if dpi <= 0 {
		dpi = 96
	}
	return (value*dpi + 48) / 96
}

func defaultSettingsLayout(dpi int) settingsLayout {
	return settingsLayout{
		ClientWidth:  scaleDPI(settingsBaseWidth, dpi),
		ClientHeight: scaleDPI(settingsBaseHeight, dpi),
		Padding:      scaleDPI(settingsBasePadding, dpi),
		Resizable:    false,
		Maximizable:  false,
	}
}

func centeredPosition(screenWidth, screenHeight, windowWidth, windowHeight int) (int, int) {
	x := (screenWidth - windowWidth) / 2
	y := (screenHeight - windowHeight) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return x, y
}

const (
	synchronize      = 0x00100000
	wmCreate         = 0x0001
	wmDestroy        = 0x0002
	wmClose          = 0x0010
	wmCommand        = 0x0111
	wmCtlColorEdit   = 0x0133
	wmCtlColorBtn    = 0x0135
	wmCtlColorStatic = 0x0138
	swShow           = 5
	swRestore        = 9
	wsExTopmost      = 0x00000008
	wsExClientEdge   = 0x00000200
	wsOverlapped     = 0x00000000
	wsCaption        = 0x00c00000
	wsSysMenu        = 0x00080000
	wsMinimizeBox    = 0x00020000
	wsChild          = 0x40000000
	wsVisible        = 0x10000000
	wsTabStop        = 0x00010000
	wsBorder         = 0x00800000
	esLeft           = 0x0000
	esNumber         = 0x2000
	bsPushButton     = 0x00000000
	ssLeft           = 0x00000000
	colorWindow      = 5
	colorBlack       = 0x000000
	colorWhite       = 0xffffff
	idiApplication   = 32512
	idcArrow         = 32512
	logPixelsX       = 88
	smCxScreen       = 0
	swpNoSize        = 0x0001
	swpNoMove        = 0x0002
	swpShowWindow    = 0x0040
	enChange         = 0x0300
	settingsIDPort   = 1001
	settingsIDStart  = 1002
	settingsIDStop   = 1003
	settingsIDOpen   = 1004
)

var hwndTopmost = ^uintptr(0)

var (
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	user32                  = syscall.NewLazyDLL("user32.dll")
	gdi32                   = syscall.NewLazyDLL("gdi32.dll")
	shell32                 = syscall.NewLazyDLL("shell32.dll")
	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procUnregisterClassW    = user32.NewProc("UnregisterClassW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procShowWindow          = user32.NewProc("ShowWindow")
	procUpdateWindow        = user32.NewProc("UpdateWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procSetWindowPos        = user32.NewProc("SetWindowPos")
	procIsIconic            = user32.NewProc("IsIconic")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procGetSysColorBrush    = user32.NewProc("GetSysColorBrush")
	procGetDC               = user32.NewProc("GetDC")
	procReleaseDC           = user32.NewProc("ReleaseDC")
	procGetClientRect       = user32.NewProc("GetClientRect")
	procAdjustWindowRectEx  = user32.NewProc("AdjustWindowRectEx")
	procLoadCursorW         = user32.NewProc("LoadCursorW")
	procLoadIconW           = user32.NewProc("LoadIconW")
	procGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	procSetWindowTextW      = user32.NewProc("SetWindowTextW")
	procGetWindowTextW      = user32.NewProc("GetWindowTextW")
	procEnableWindow        = user32.NewProc("EnableWindow")
	procGetDeviceCaps       = gdi32.NewProc("GetDeviceCaps")
	procSetTextColor        = gdi32.NewProc("SetTextColor")
	procSetBkColor          = gdi32.NewProc("SetBkColor")
	procGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
	openProcess             = kernel32.NewProc("OpenProcess")
	waitSingleObject        = kernel32.NewProc("WaitForSingleObject")
	closeHandle             = kernel32.NewProc("CloseHandle")
	procShellExecuteW       = shell32.NewProc("ShellExecuteW")
)

type point struct{ X, Y int32 }
type rect struct{ Left, Top, Right, Bottom int32 }
type msg struct {
	HWnd, Message, WParam, LParam uintptr
	Time                          uint32
	Pt                            point
	Private                       uint32
}
type wndClassEx struct {
	Size, Style          uint32
	WndProc, ClsExtra    uintptr
	WndExtra, Instance   uintptr
	Icon, Cursor         uintptr
	Background, MenuName uintptr
	ClassName, IconSmall uintptr
}

type settingsControls struct {
	statusLabel uintptr
	portEdit    uintptr
	portState   uintptr
	startButton uintptr
	stopButton  uintptr
	openButton  uintptr
}

var settingsNative = struct {
	sync.Mutex
	hwnd     uintptr
	controls settingsControls
	done     chan struct{}
	closing  bool
}{}

var settingsWndProc = syscall.NewCallback(settingsWindowProc)

func showSettingsWindow() {
	settingsNative.Lock()
	if settingsNative.hwnd != 0 {
		hwnd := settingsNative.hwnd
		settingsNative.Unlock()
		iconic, _, _ := procIsIconic.Call(hwnd)
		if iconic != 0 {
			procShowWindow.Call(hwnd, swRestore)
		}
		refreshSettingsControls()
		procSetWindowPos.Call(hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpShowWindow)
		procSetForegroundWindow.Call(hwnd)
		return
	}
	if settingsNative.done != nil || settingsNative.closing {
		settingsNative.Unlock()
		return
	}
	settingsNative.done = make(chan struct{})
	done := settingsNative.done
	settingsNative.Unlock()
	go settingsWindowThread(done)
}

func closeSettingsWindow() {
	settingsNative.Lock()
	done, hwnd := settingsNative.done, settingsNative.hwnd
	if done == nil {
		settingsNative.Unlock()
		return
	}
	settingsNative.closing = true
	settingsNative.Unlock()
	if hwnd != 0 {
		procPostMessageW.Call(hwnd, wmClose, 0, 0)
	}
	<-done
}

func settingsWindowThread(done chan struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer func() {
		settingsNative.Lock()
		settingsNative.hwnd = 0
		settingsNative.controls = settingsControls{}
		settingsNative.done = nil
		settingsNative.closing = false
		close(done)
		settingsNative.Unlock()
	}()

	instance, _, _ := procGetModuleHandleW.Call(0)
	className, _ := syscall.UTF16PtrFromString("BeeGoAISettingsWindow")
	title, _ := syscall.UTF16PtrFromString(PluginName + " 设置")
	cursor, _, _ := procLoadCursorW.Call(0, idcArrow)
	icon, _, _ := procLoadIconW.Call(0, idiApplication)
	class := wndClassEx{Size: uint32(unsafe.Sizeof(wndClassEx{})), WndProc: settingsWndProc, Instance: instance, Icon: icon, Cursor: cursor, Background: colorWindow + 1, ClassName: uintptr(unsafe.Pointer(className)), IconSmall: icon}
	atom, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))
	if atom == 0 {
		return
	}
	defer procUnregisterClassW.Call(uintptr(unsafe.Pointer(className)), instance)

	dpi := settingsDPI()
	layout := defaultSettingsLayout(dpi)
	style := uintptr(wsOverlapped | wsCaption | wsSysMenu | wsMinimizeBox)
	windowRect := rect{Right: int32(layout.ClientWidth), Bottom: int32(layout.ClientHeight)}
	procAdjustWindowRectEx.Call(uintptr(unsafe.Pointer(&windowRect)), style, 0, 0)
	width, height := int(windowRect.Right-windowRect.Left), int(windowRect.Bottom-windowRect.Top)
	screenW, _, _ := procGetSystemMetrics.Call(smCxScreen)
	screenH, _, _ := procGetSystemMetrics.Call(1)
	x, y := centeredPosition(int(screenW), int(screenH), width, height)
	hwnd, _, _ := procCreateWindowExW.Call(wsExTopmost, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)), style, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0, 0, instance, 0)
	if hwnd == 0 {
		return
	}
	settingsNative.Lock()
	settingsNative.hwnd = hwnd
	settingsNative.Unlock()
	procShowWindow.Call(hwnd, swShow)
	procSetWindowPos.Call(hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpShowWindow)
	procSetForegroundWindow.Call(hwnd)
	procUpdateWindow.Call(hwnd)
	var message msg
	for {
		result, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
}

func settingsDPI() int {
	dc, _, _ := procGetDC.Call(0)
	if dc == 0 {
		return 96
	}
	defer procReleaseDC.Call(0, dc)
	dpi, _, _ := procGetDeviceCaps.Call(dc, logPixelsX)
	if dpi == 0 {
		return 96
	}
	return int(dpi)
}

func settingsWindowProc(hwnd uintptr, message uint32, wparam, lparam uintptr) uintptr {
	switch message {
	case wmCreate:
		createSettingsControls(hwnd)
		refreshSettingsControls()
		return 0
	case wmCommand:
		handleSettingsCommand(wparam)
		return 0
	case wmCtlColorStatic, wmCtlColorEdit, wmCtlColorBtn:
		return whiteControlBrush(wparam)
	case wmClose:
		procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	result, _, _ := procDefWindowProcW.Call(hwnd, uintptr(message), wparam, lparam)
	return result
}

func whiteControlBrush(hdc uintptr) uintptr {
	procSetTextColor.Call(hdc, colorBlack)
	procSetBkColor.Call(hdc, colorWhite)
	brush, _, _ := procGetSysColorBrush.Call(colorWindow)
	return brush
}

func createSettingsControls(hwnd uintptr) {
	dpi := settingsDPI()
	p := scaleDPI(settingsBasePadding, dpi)
	labelW := scaleDPI(120, dpi)
	editW := scaleDPI(120, dpi)
	lineH := scaleDPI(26, dpi)
	gap := scaleDPI(10, dpi)
	y := p
	status := createChild(hwnd, "STATIC", "", ssLeft, 0, p, y, scaleDPI(500, dpi), lineH, 0)
	y += lineH + gap
	createChild(hwnd, "STATIC", "HTTP端口：", ssLeft, 0, p, y+scaleDPI(4, dpi), labelW, lineH, 0)
	portEdit := createChild(hwnd, "EDIT", strconv.Itoa(defaultHTTPPort), wsBorder|esLeft|esNumber, wsExClientEdge, p+labelW, y, editW, lineH, settingsIDPort)
	portState := createChild(hwnd, "STATIC", "", ssLeft, 0, p+labelW+editW+gap, y+scaleDPI(4, dpi), scaleDPI(250, dpi), lineH, 0)
	y += lineH + scaleDPI(22, dpi)
	startButton := createChild(hwnd, "BUTTON", "启动HTTP服务", bsPushButton|wsTabStop, 0, p, y, scaleDPI(126, dpi), lineH+scaleDPI(8, dpi), settingsIDStart)
	stopButton := createChild(hwnd, "BUTTON", "停止HTTP服务", bsPushButton|wsTabStop, 0, p+scaleDPI(138, dpi), y, scaleDPI(126, dpi), lineH+scaleDPI(8, dpi), settingsIDStop)
	openButton := createChild(hwnd, "BUTTON", "打开网址", bsPushButton|wsTabStop, 0, p+scaleDPI(276, dpi), y, scaleDPI(104, dpi), lineH+scaleDPI(8, dpi), settingsIDOpen)
	settingsNative.Lock()
	settingsNative.controls = settingsControls{statusLabel: status, portEdit: portEdit, portState: portState, startButton: startButton, stopButton: stopButton, openButton: openButton}
	settingsNative.Unlock()
}

func createChild(parent uintptr, className, text string, style, exStyle uintptr, x, y, width, height, id int) uintptr {
	classPtr, _ := syscall.UTF16PtrFromString(className)
	textPtr, _ := syscall.UTF16PtrFromString(text)
	hwnd, _, _ := procCreateWindowExW.Call(
		exStyle,
		uintptr(unsafe.Pointer(classPtr)),
		uintptr(unsafe.Pointer(textPtr)),
		wsChild|wsVisible|style,
		uintptr(x),
		uintptr(y),
		uintptr(width),
		uintptr(height),
		parent,
		uintptr(id),
		0,
		0,
	)
	return hwnd
}

func handleSettingsCommand(wparam uintptr) {
	id := int(wparam & 0xffff)
	notify := uint32((wparam >> 16) & 0xffff)
	switch id {
	case settingsIDPort:
		if notify == enChange {
			updatePortState()
		}
	case settingsIDStart:
		startSettingsHTTP()
	case settingsIDStop:
		stopSettingsHTTP()
	case settingsIDOpen:
		openSettingsURL()
	}
}

func refreshSettingsControls() {
	settingsNative.Lock()
	controls := settingsNative.controls
	settingsNative.Unlock()
	if controls.statusLabel == 0 {
		return
	}
	status := HTTPServiceStatus{Port: defaultHTTPPort, URL: ConfigURL(defaultHTTPPort), Config: DefaultAIConfig()}
	if service := currentHTTPService(); service != nil {
		status = service.Status()
	}
	setWindowText(controls.statusLabel, "HTTP服务："+runningText(status.Running))
	setWindowText(controls.portEdit, strconv.Itoa(status.Config.Port))
	updatePortState()
	enableWindow(controls.startButton, !status.Running)
	enableWindow(controls.stopButton, status.Running)
	enableWindow(controls.openButton, status.Running)
}

func updatePortState() {
	settingsNative.Lock()
	controls := settingsNative.controls
	settingsNative.Unlock()
	if controls.portState == 0 {
		return
	}
	port, ok := readPortEdit()
	if !ok {
		setWindowText(controls.portState, "端口无效")
		return
	}
	if service := currentHTTPService(); service != nil {
		status := service.Status()
		if status.Running && status.Port == port {
			setWindowText(controls.portState, "当前服务使用中")
			return
		}
	}
	if IsPortAvailable(port) {
		setWindowText(controls.portState, "端口可用")
	} else {
		setWindowText(controls.portState, "端口不可用")
	}
}

func startSettingsHTTP() {
	service := currentHTTPService()
	if service == nil {
		if dataDir, err := pluginDataDir(); err == nil {
			ensurePluginServices(dataDir, nil)
			service = currentHTTPService()
		}
		if service == nil {
			setStatusText("HTTP服务：初始化失败")
			return
		}
	}
	port, ok := readPortEdit()
	if !ok {
		setStatusText("HTTP服务：端口无效")
		updatePortState()
		return
	}
	if !IsPortAvailable(port) {
		setStatusText("HTTP服务：端口不可用")
		updatePortState()
		return
	}
	if err := service.Start(port); err != nil {
		setStatusText("HTTP服务：启动失败 - " + err.Error())
		updatePortState()
		return
	}
	refreshSettingsControls()
}

func stopSettingsHTTP() {
	if service := currentHTTPService(); service != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = service.Stop(ctx)
		cancel()
	}
	refreshSettingsControls()
}

func openSettingsURL() {
	status := HTTPServiceStatus{Port: defaultHTTPPort}
	if service := currentHTTPService(); service != nil {
		status = service.Status()
	}
	operation, _ := syscall.UTF16PtrFromString("open")
	target, _ := syscall.UTF16PtrFromString(ConfigURL(status.Port))
	procShellExecuteW.Call(0, uintptr(unsafe.Pointer(operation)), uintptr(unsafe.Pointer(target)), 0, 0, swShow)
}

func readPortEdit() (int, bool) {
	settingsNative.Lock()
	portEdit := settingsNative.controls.portEdit
	settingsNative.Unlock()
	if portEdit == 0 {
		return defaultHTTPPort, true
	}
	var buffer [32]uint16
	procGetWindowTextW.Call(portEdit, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	text := syscall.UTF16ToString(buffer[:])
	port, err := strconv.Atoi(text)
	if err != nil || port <= 0 || port > 65535 {
		return 0, false
	}
	return port, true
}

func setStatusText(text string) {
	settingsNative.Lock()
	status := settingsNative.controls.statusLabel
	settingsNative.Unlock()
	if status != 0 {
		setWindowText(status, text)
	}
}

func setWindowText(hwnd uintptr, text string) {
	value, _ := syscall.UTF16PtrFromString(text)
	procSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(value)))
}

func enableWindow(hwnd uintptr, enabled bool) {
	value := uintptr(0)
	if enabled {
		value = 1
	}
	procEnableWindow.Call(hwnd, value)
}

func runningText(running bool) string {
	if running {
		return "运行中"
	}
	return "未启动"
}

func startHostWatcher(hostPID uint32) {
	handle, _, _ := openProcess.Call(synchronize, 0, uintptr(hostPID))
	if handle == 0 {
		return
	}
	go func() {
		defer closeHandle.Call(handle)
		waitSingleObject.Call(handle, 0xffffffff)
		os.Exit(0)
	}()
}
