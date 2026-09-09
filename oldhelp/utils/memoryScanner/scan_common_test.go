package memoryScanner

import (
	"errors"
	"io"
	"reflect"
	"testing"
)

func testAuthz(nonce string) string {
	return authzPattern + "123456789012345678901234" + "&nonce=" + nonce
}

func TestScanRegionChunked(t *testing.T) {
	t.Run("zero region returns empty candidates", func(t *testing.T) {
		candidates, err := scanRegionChunked(0, func(offset uintptr, dst []byte) (int, error) {
			t.Fatalf("readChunk called for zero region: offset=%d len=%d", offset, len(dst))
			return 0, nil
		})
		if err != nil {
			t.Fatalf("scanRegionChunked() error = %v", err)
		}
		if len(candidates) != 0 {
			t.Fatalf("scanRegionChunked() candidates = %#v, want empty", candidates)
		}
	})

	t.Run("finds authz across chunk boundary", func(t *testing.T) {
		authz := testAuthz("987654321")
		data := make([]byte, scanChunkSize+len(authz)+8)
		start := scanChunkSize - 6
		copy(data[start:], []byte(authz))
		copy(data[start+len(authz):], []byte(" trailer"))

		candidates, err := scanRegionChunked(uintptr(len(data)), func(offset uintptr, dst []byte) (int, error) {
			n := copy(dst, data[offset:])
			return n, nil
		})
		if err != nil {
			t.Fatalf("scanRegionChunked() error = %v", err)
		}

		want := map[string]int{authz: 1}
		if !reflect.DeepEqual(candidates, want) {
			t.Fatalf("scanRegionChunked() candidates = %#v, want %#v", candidates, want)
		}
	})

	t.Run("read error returns immediately", func(t *testing.T) {
		wantErr := errors.New("boom")
		_, err := scanRegionChunked(32, func(offset uintptr, dst []byte) (int, error) {
			return 0, wantErr
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("scanRegionChunked() error = %v, want %v", err, wantErr)
		}
	})

	t.Run("zero bytes before region end is unexpected eof", func(t *testing.T) {
		_, err := scanRegionChunked(32, func(offset uintptr, dst []byte) (int, error) {
			return 0, nil
		})
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("scanRegionChunked() error = %v, want %v", err, io.ErrUnexpectedEOF)
		}
	})
}

func TestScanChunk(t *testing.T) {
	t.Run("counts complete authz and keeps tail", func(t *testing.T) {
		authz := testAuthz("111222333")
		candidates := make(map[string]int)
		chunk := []byte("prefix-" + authz + "-suffix")

		carry := scanChunk(nil, chunk, false, candidates)

		if got := candidates[authz]; got != 1 {
			t.Fatalf("candidate count = %d, want 1", got)
		}
		wantCarry := []byte(chunk[len(chunk)-len(authzPattern)+1:])
		if !reflect.DeepEqual(carry, wantCarry) {
			t.Fatalf("carry = %q, want %q", carry, wantCarry)
		}
	})

	t.Run("returns incomplete authz as carry", func(t *testing.T) {
		authz := testAuthz("444555666")
		partial := authz[:len(authz)-2]
		candidates := make(map[string]int)

		carry := scanChunk(nil, []byte("noise"+partial), false, candidates)

		if len(candidates) != 0 {
			t.Fatalf("candidates = %#v, want empty", candidates)
		}
		if got := string(carry); got != partial {
			t.Fatalf("carry = %q, want %q", got, partial)
		}
	})

	t.Run("final chunk drops incomplete carry", func(t *testing.T) {
		partial := authzPattern + "123456789012345678901234&nonce="
		candidates := make(map[string]int)

		carry := scanChunk(nil, []byte(partial), true, candidates)

		if carry != nil {
			t.Fatalf("carry = %q, want nil", carry)
		}
		if len(candidates) != 0 {
			t.Fatalf("candidates = %#v, want empty", candidates)
		}
	})
}

func TestExtractAuthz(t *testing.T) {
	authz := testAuthz("1234567890")

	tests := []struct {
		name       string
		buf        string
		final      bool
		wantAuthz  string
		wantStatus authzStatus
	}{
		{
			name:       "complete authz stops at first nondigit",
			buf:        authz + "&rest=true",
			wantAuthz:  authz,
			wantStatus: authzComplete,
		},
		{
			name:       "short buffer incomplete before final",
			buf:        authz[:len(authzPattern)+10],
			wantStatus: authzIncomplete,
		},
		{
			name:       "missing nonce prefix invalid",
			buf:        authzPattern + "123456789012345678901234&token=12",
			wantStatus: authzInvalid,
		},
		{
			name:       "missing nonce digits incomplete before final",
			buf:        authzPattern + "123456789012345678901234&nonce=",
			wantStatus: authzIncomplete,
		},
		{
			name:       "missing nonce digits invalid at final",
			buf:        authzPattern + "123456789012345678901234&nonce=",
			final:      true,
			wantStatus: authzInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAuthz, gotStatus := extractAuthz([]byte(tt.buf), tt.final)
			if gotAuthz != tt.wantAuthz || gotStatus != tt.wantStatus {
				t.Fatalf("extractAuthz() = (%q, %v), want (%q, %v)", gotAuthz, gotStatus, tt.wantAuthz, tt.wantStatus)
			}
		})
	}
}

func TestFindConfident(t *testing.T) {
	if got := findConfident(map[string]int{"low": authzConfidence - 1, "high": authzConfidence}); got != "high" {
		t.Fatalf("findConfident() = %q, want %q", got, "high")
	}
	if got := findConfident(map[string]int{"low": authzConfidence - 1}); got != "" {
		t.Fatalf("findConfident() = %q, want empty", got)
	}
}

func TestMergeCandidates(t *testing.T) {
	dst := map[string]int{"a": 1, "b": 2}
	src := map[string]int{"b": 3, "c": 4}

	mergeCandidates(dst, src)

	want := map[string]int{"a": 1, "b": 5, "c": 4}
	if !reflect.DeepEqual(dst, want) {
		t.Fatalf("mergeCandidates() dst = %#v, want %#v", dst, want)
	}
}
