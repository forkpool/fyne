// +build !ci

package efl

// #cgo pkg-config: ecore ecore-evas ecore-input evas
// #include <Ecore.h>
// #include <Ecore_Evas.h>
// #include <Ecore_Input.h>
// #include <Evas.h>
//
// void onWindowResize_cgo(Ecore_Evas *);
// void onWindowMove_cgo(Ecore_Evas *);
// void onWindowClose_cgo(Ecore_Evas *);
//
// void onWindowKeyDown_cgo(Ecore_Window, void *);
import "C"

import "log"
import "os"
import "strconv"
import "unsafe"

import "github.com/fyne-io/fyne/api/ui"
import "github.com/fyne-io/fyne/api/ui/input"

type window struct {
	ee     *C.Ecore_Evas
	canvas ui.Canvas
	driver *eFLDriver
	master bool
}

var windows = make(map[*C.Ecore_Evas]*window)

func (w *window) Title() string {
	return C.GoString(C.ecore_evas_title_get(w.ee))
}

func (w *window) SetTitle(title string) {
	C.ecore_evas_title_set(w.ee, C.CString(title))
}

func (w *window) Fullscreen() bool {
	return C.ecore_evas_fullscreen_get(w.ee) != 0
}

func (w *window) SetFullscreen(full bool) {
	if full {
		C.ecore_evas_fullscreen_set(w.ee, 1)
	} else {
		C.ecore_evas_fullscreen_set(w.ee, 0)
	}
}

func (w *window) Show() {
	C.ecore_evas_show(w.ee)

	if len(windows) == 1 {
		w.master = true
		initEFL()
	}
}

func (w *window) Hide() {
	C.ecore_evas_hide(w.ee)
}

func (w *window) Close() {
	w.Hide()

	if w.master || len(windows) == 1 {
		DoQuit()
	} else {
		delete(windows, w.ee)
	}
}

func (w *window) Canvas() ui.Canvas {
	return w.canvas
}

func scaleByDPI(w *window) float32 {
	xdpi := C.int(0)

	env := os.Getenv("FYNE_SCALE")
	if env != "" {
		scale, _ := strconv.ParseFloat(env, 32)
		return float32(scale)
	}
	C.ecore_evas_screen_dpi_get(w.ee, &xdpi, nil)
	if xdpi > 250 {
		return float32(1.5)
	} else if xdpi > 120 {
		return float32(1.2)
	}

	return float32(1.0)
}

//export onWindowResize
func onWindowResize(ee *C.Ecore_Evas) {
	var ww, hh C.int
	C.ecore_evas_geometry_get(ee, nil, nil, &ww, &hh)

	w := windows[ee]

	canvas := w.canvas.(*eflCanvas)
	canvas.size = ui.NewSize(int(float32(ww)/canvas.Scale()), int(float32(hh)/canvas.Scale()))
	canvas.Refresh(canvas.content)
}

//export onWindowMove
func onWindowMove(ee *C.Ecore_Evas) {
	w := windows[ee]
	canvas := w.canvas.(*eflCanvas)

	scale := scaleByDPI(w)
	if scale != canvas.Scale() {
		canvas.SetScale(scaleByDPI(w))
	}
}

//export onWindowClose
func onWindowClose(ee *C.Ecore_Evas) {
	windows[ee].Close()
}

//export onWindowKeyDown
func onWindowKeyDown(ew C.Ecore_Window, info *C.Ecore_Event_Key) {
	if ew == 0 {
		log.Println("Keystroke missing window")
		return
	}

	var w *window
	for _, win := range windows {
		if C.ecore_evas_window_get(win.ee) == ew {
			w = win
		}
	}

	if w == nil {
		log.Println("Window not found")
		return
	}
	canvas := w.canvas.(*eflCanvas)

	if canvas.focussed == nil && canvas.onKeyDown == nil {
		return
	}

	ev := new(ui.KeyEvent)
	ev.String = C.GoString(info.string)
	ev.Name = C.GoString(info.keyname)
	ev.Code = input.KeyCode(int(info.keycode))
	if (info.modifiers & C.ECORE_EVENT_MODIFIER_SHIFT) != 0 {
		ev.Modifiers |= input.ShiftModifier
	}
	if (info.modifiers & C.ECORE_EVENT_MODIFIER_CTRL) != 0 {
		ev.Modifiers |= input.ControlModifier
	}
	if (info.modifiers & C.ECORE_EVENT_MODIFIER_ALT) != 0 {
		ev.Modifiers |= input.AltModifier
	}

	if canvas.focussed != nil {
		canvas.focussed.OnKeyDown(ev)
	}
	if canvas.onKeyDown != nil {
		canvas.onKeyDown(ev)
	}
}

func (d *eFLDriver) CreateWindow(title string) ui.Window {
	engine := oSEngineName()

	C.evas_init()
	C.ecore_init()
	C.ecore_evas_init()

	evas := C.ecore_evas_new(C.CString(engine), 0, 0, 10, 10, nil)
	if evas == nil {
		log.Fatalln("Unable to create canvas, perhaps missing module for", engine)
	}

	w := &window{
		ee:     evas,
		driver: d,
	}
	w.SetTitle(title)
	oSWindowInit(w)
	c := &eflCanvas{
		evas:   C.ecore_evas_get(evas),
		scale:  1.0,
		window: w,
	}
	w.canvas = c
	windows[w.ee] = w
	C.ecore_evas_callback_resize_set(w.ee, (C.Ecore_Evas_Event_Cb)(unsafe.Pointer(C.onWindowResize_cgo)))
	C.ecore_evas_callback_move_set(w.ee, (C.Ecore_Evas_Event_Cb)(unsafe.Pointer(C.onWindowMove_cgo)))
	C.ecore_evas_callback_delete_request_set(w.ee, (C.Ecore_Evas_Event_Cb)(unsafe.Pointer(C.onWindowClose_cgo)))

	C.ecore_event_handler_add(C.ECORE_EVENT_KEY_DOWN, (C.Ecore_Event_Handler_Cb)(unsafe.Pointer(C.onWindowKeyDown_cgo)), nil)

	c.SetContent(new(ui.Container))
	return w
}

func (d *eFLDriver) AllWindows() []ui.Window {
	wins := make([]ui.Window, 0, len(windows))

	for _, win := range windows {
		wins = append(wins, win)
	}

	return wins
}