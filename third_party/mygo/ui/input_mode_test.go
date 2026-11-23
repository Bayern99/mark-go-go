package ui

import "testing"

func TestContextKeyboardInputTracksInputModality(t *testing.T) {
	keyboard := true
	tt := NewTester(func(c *Context) {
		keyboard = c.KeyboardInput()
		Row(c).Children(func() {
			ButtonBase(c.Key("first")).Children(func() { Text(c, "First") })
			ButtonBase(c.Key("second")).Children(func() { Text(c, "Second") })
		})
	}, 240, 100)
	if keyboard {
		t.Fatal("KeyboardInput starts true before any input")
	}

	tt.Key(0, KeyTab)
	if !keyboard {
		t.Fatal("KeyboardInput is false after a key press")
	}
	if err := tt.Click("First"); err != nil {
		t.Fatal(err)
	}
	if keyboard {
		t.Fatal("KeyboardInput stays true after a pointer press on the focused control")
	}

	tt.Key(0, KeyRight)
	if !keyboard {
		t.Fatal("KeyboardInput is false after a key press that keeps the focus in place")
	}
	if err := tt.Click("Second"); err != nil {
		t.Fatal(err)
	}
	if keyboard {
		t.Fatal("KeyboardInput stays true after pointer input moves the focus")
	}
}
