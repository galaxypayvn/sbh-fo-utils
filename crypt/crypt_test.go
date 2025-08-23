package crypt

import (
	"crypto/rand"
	"fmt"
	"testing"
)

func TestEncrypt(t *testing.T) {
	type args struct {
		text   string
		secret []byte
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Encrypt simple text with 32-byte key",
			args: args{
				text:   "Hello, World!",
				secret: []byte("12345678901234567890123456789012"),
			},
			wantErr: false,
		},
		{
			name: "Encrypt empty string with 32-byte key",
			args: args{
				text:   "",
				secret: []byte("12345678901234567890123456789012"),
			},
			wantErr: false,
		},
		{
			name: "Encrypt with 16-byte key",
			args: args{
				text:   "Test message",
				secret: []byte("1234567890123456"),
			},
			wantErr: false,
		},
		{
			name: "Encrypt with 24-byte key",
			args: args{
				text:   "Test message",
				secret: []byte("123456789012345678901234"),
			},
			wantErr: false,
		},
		{
			name: "Encrypt with invalid key size",
			args: args{
				text:   "Test message",
				secret: []byte("invalid"),
			},
			wantErr: true,
		},
		{
			name: "Encrypt unicode text",
			args: args{
				text:   "Xin chào! 你好! Здравствуй!",
				secret: []byte("12345678901234567890123456789012"),
			},
			wantErr: false,
		},
		{
			name: "Encrypt long text",
			args: args{
				text:   "This is a very long text that should be encrypted properly without any issues. It contains multiple sentences and should test the encryption of larger data sizes.",
				secret: []byte("12345678901234567890123456789012"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Encrypt(tt.args.text, tt.args.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("Encrypt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got == "" {
					t.Errorf("Encrypt() returned empty string when expecting encrypted text")
				}
				if got == tt.args.text {
					t.Errorf("Encrypt() returned plaintext, expected encrypted text")
				}
			}
		})
	}
}

func TestDecrypt(t *testing.T) {
	type args struct {
		text   string
		secret []byte
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "Decrypt with invalid hex string",
			args: args{
				text:   "invalid_hex",
				secret: []byte("12345678901234567890123456789012"),
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "Decrypt with invalid key size",
			args: args{
				text:   "abcdef123456",
				secret: []byte("invalid"),
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "Decrypt with too short ciphertext",
			args: args{
				text:   "abc",
				secret: []byte("12345678901234567890123456789012"),
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decrypt(tt.args.text, tt.args.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("Decrypt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Decrypt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	type args struct {
		text   string
		secret []byte
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Round trip simple text",
			args: args{
				text:   "Hello, World!",
				secret: []byte("12345678901234567890123456789012"),
			},
		},
		{
			name: "Round trip empty string",
			args: args{
				text:   "",
				secret: []byte("12345678901234567890123456789012"),
			},
		},
		{
			name: "Round trip unicode text",
			args: args{
				text:   "Xin chào! 你好! Здравствуй!",
				secret: []byte("12345678901234567890123456789012"),
			},
		},
		{
			name: "Round trip with special characters",
			args: args{
				text:   "!@#$%^&*()_+-=[]{}|;':\",./<>?",
				secret: []byte("12345678901234567890123456789012"),
			},
		},
		{
			name: "Round trip with newlines and tabs",
			args: args{
				text:   "Line 1\nLine 2\n\tTabbed line",
				secret: []byte("12345678901234567890123456789012"),
			},
		},
		{
			name: "Round trip with 16-byte key",
			args: args{
				text:   "Test with 16-byte key",
				secret: []byte("1234567890123456"),
			},
		},
		{
			name: "Round trip with 24-byte key",
			args: args{
				text:   "Test with 24-byte key",
				secret: []byte("123456789012345678901234"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := Encrypt(tt.args.text, tt.args.secret)
			if err != nil {
				t.Errorf("Encrypt() error = %v", err)
				return
			}

			decrypted, err := Decrypt(encrypted, tt.args.secret)
			if err != nil {
				t.Errorf("Decrypt() error = %v", err)
				return
			}

			if decrypted != tt.args.text {
				t.Errorf("Round trip failed: original = %q, decrypted = %q", tt.args.text, decrypted)
			}
		})
	}
}

func TestEncryptDecryptWithDifferentKeys(t *testing.T) {
	originalText := "Secret message"
	key1 := []byte("12345678901234567890123456789012")
	key2 := []byte("09876543210987654321098765432109")

	encrypted, err := Encrypt(originalText, key1)
	if err != nil {
		t.Errorf("Encrypt() error = %v", err)
		return
	}

	_, err = Decrypt(encrypted, key2)
	if err == nil {
		t.Error("Decrypt() should fail when using different key")
	}
}

func TestEncryptNonDeterministic(t *testing.T) {
	text := "Test message"
	secret := []byte("12345678901234567890123456789012")

	encrypted1, err := Encrypt(text, secret)
	if err != nil {
		t.Errorf("First Encrypt() error = %v", err)
		return
	}

	encrypted2, err := Encrypt(text, secret)
	if err != nil {
		t.Errorf("Second Encrypt() error = %v", err)
		return
	}

	if encrypted1 == encrypted2 {
		t.Error("Encrypt() should produce different ciphertext for the same input (due to random nonce)")
	}

	decrypted1, err := Decrypt(encrypted1, secret)
	if err != nil {
		t.Errorf("Decrypt encrypted1 error = %v", err)
		return
	}

	decrypted2, err := Decrypt(encrypted2, secret)
	if err != nil {
		t.Errorf("Decrypt encrypted2 error = %v", err)
		return
	}

	if decrypted1 != text || decrypted2 != text {
		t.Errorf("Both decryptions should equal original text: got %q and %q, want %q", decrypted1, decrypted2, text)
	}
}

func BenchmarkEncrypt(b *testing.B) {
	text := "This is a test message for benchmarking encryption performance"
	secret := []byte("12345678901234567890123456789012")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Encrypt(text, secret)
		if err != nil {
			b.Fatalf("Encrypt() error = %v", err)
		}
	}
}

func BenchmarkDecrypt(b *testing.B) {
	text := "This is a test message for benchmarking decryption performance"
	secret := []byte("12345678901234567890123456789012")

	encrypted, err := Encrypt(text, secret)
	if err != nil {
		b.Fatalf("Setup Encrypt() error = %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Decrypt(encrypted, secret)
		if err != nil {
			b.Fatalf("Decrypt() error = %v", err)
		}
	}
}

func generateRandomKey(size int) []byte {
	key := make([]byte, size)
	rand.Read(key)
	return key
}

func TestEncryptWithRandomKeys(t *testing.T) {
	text := "Random key test"
	
	keySizes := []int{16, 24, 32}
	for _, size := range keySizes {
		t.Run(fmt.Sprintf("KeySize%d", size), func(t *testing.T) {
			key := generateRandomKey(size)
			
			encrypted, err := Encrypt(text, key)
			if err != nil {
				t.Errorf("Encrypt() with random %d-byte key error = %v", size, err)
				return
			}
			
			decrypted, err := Decrypt(encrypted, key)
			if err != nil {
				t.Errorf("Decrypt() with random %d-byte key error = %v", size, err)
				return
			}
			
			if decrypted != text {
				t.Errorf("Round trip with random %d-byte key failed: got %q, want %q", size, decrypted, text)
			}
		})
	}
}