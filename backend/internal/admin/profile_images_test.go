package admin

import (
	"bytes"
	"image"
	"image/jpeg"
	"testing"
)

func TestProfilePhotoValidation(t *testing.T) {
	var zero decimalIDInput
	var input bytes.Buffer
	if err := jpeg.Encode(&input, image.NewRGBA(image.Rect(0, 0, 512, 512)), nil); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name          string
		data          []byte
		previous      *decimalIDInput
		width, height int
		valid         bool
	}{
		{"avatar", input.Bytes(), &zero, 512, 512, true},
		{"wrong aspect", input.Bytes(), &zero, 1200, 675, false},
		{"truncated", input.Bytes()[:len(input.Bytes())/2], &zero, 512, 512, false},
		{"missing revision", input.Bytes(), nil, 512, 512, false},
		{"empty", nil, &zero, 512, 512, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			change := profilePhotoChange{PreviousID: test.previous, Data: test.data}
			if err := change.validate(test.width, test.height); (err == nil) != test.valid {
				t.Fatalf("valid=%v, err=%v", test.valid, err)
			}
		})
	}
}
