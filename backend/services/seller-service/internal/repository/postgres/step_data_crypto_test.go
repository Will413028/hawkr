package postgres

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/crypto"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres/gen"
)

// realIDCardNo / realBankAccount are the shapes actually observed in dev and
// prod: 10-char id card, 8-14 digit account.
const (
	realIDCardNo    = "A123456789"
	realBankAccount = "00012345678901"
)

// stepDataValue pulls one raw key out of a step_data document. Tests assert on
// what is bound to the SQL parameter, so they must read the JSON, not the domain.
func stepDataValue(t *testing.T, raw []byte, key string) (string, bool) {
	t.Helper()
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("step_data is not a JSON object: %v", err)
	}
	encoded, ok := fields[key]
	if !ok {
		return "", false
	}
	var value string
	if err := json.Unmarshal(encoded, &value); err != nil {
		return "", false
	}
	return value, true
}

// TestSensitiveStepDataKeysCoversEveryProducerSpelling pins the key list against
// the producers. If a frontend adds a spelling and nobody updates the list, the
// value silently goes back to being stored in plaintext — there is no other
// signal, so this list is the whole defence and deserves an explicit test.
func TestSensitiveStepDataKeysCoversEveryProducerSpelling(t *testing.T) {
	for _, key := range []string{
		"idNumber",     // hawkr_seller wizard autosave
		"id_card_no",   // hawkr_seller pre-submit save
		"idCardNo",     // hawkr-website form state
		"bankAccount",  // seller autosave + website form state
		"bank_account", // seller pre-submit save
	} {
		if _, ok := sensitiveStepDataKeys[key]; !ok {
			t.Errorf("producer spelling %q is not in sensitiveStepDataKeys — values under this key would be stored in plaintext", key)
		}
	}
	if len(sensitiveStepDataKeys) != 5 {
		t.Errorf("sensitiveStepDataKeys has %d entries, expected 5; if a key was added deliberately, extend this test and say why in the list's doc comment", len(sensitiveStepDataKeys))
	}
}

// TestStepDataStoredValueIsNotPlaintext is the "encryption actually happened"
// proof required of this change: it reads the value bound to the SQL parameter
// and asserts it does not equal the plaintext. A round-trip test alone would
// pass even if both sides were no-ops, so this is the one that must go red if
// encryption is ever removed from a write choke point.
func TestStepDataStoredValueIsNotPlaintext(t *testing.T) {
	enc := newTestCipher(t)
	// One document per producer shape, so every spelling is exercised end to end.
	for _, c := range []struct {
		name        string
		stepData    string
		idKey       string
		bankKey     string
		untouched   string
		untouchedTo string
	}{
		{
			name:        "seller wizard autosave (camelCase)",
			stepData:    `{"name":"陳先生","idNumber":"` + realIDCardNo + `","bankAccount":"` + realBankAccount + `","bankCode":"812","agreedToTerms":true}`,
			idKey:       "idNumber",
			bankKey:     "bankAccount",
			untouched:   "bankCode",
			untouchedTo: "812",
		},
		{
			name:        "seller pre-submit save (snake_case)",
			stepData:    `{"real_name":"陳先生","id_card_no":"` + realIDCardNo + `","bank_account":"` + realBankAccount + `","bank_code":"812"}`,
			idKey:       "id_card_no",
			bankKey:     "bank_account",
			untouched:   "bank_code",
			untouchedTo: "812",
		},
		{
			name:        "website form state (idCardNo)",
			stepData:    `{"realName":"陳先生","idCardNo":"` + realIDCardNo + `","bankAccount":"` + realBankAccount + `","bankName":"台新"}`,
			idKey:       "idCardNo",
			bankKey:     "bankAccount",
			untouched:   "bankName",
			untouchedTo: "台新",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			app := &domain.CertificationApplication{
				ID:       1,
				Status:   domain.CertStatusDraft,
				IDCardNo: realIDCardNo,
				StepData: json.RawMessage(c.stepData),
			}

			for _, w := range []struct {
				what   string
				params func() ([]byte, error)
			}{
				{"CreateApplicationParams", func() ([]byte, error) {
					p, err := toCreateApplicationParams(app, enc)
					return p.StepData, err
				}},
				{"UpdateApplicationParams", func() ([]byte, error) {
					p, err := toUpdateApplicationParams(app, enc)
					return p.StepData, err
				}},
			} {
				stored, err := w.params()
				if err != nil {
					t.Fatalf("%s: %v", w.what, err)
				}

				// The whole point: the bytes heading for the column must not
				// contain the account number or the id card number anywhere.
				if strings.Contains(string(stored), realBankAccount) {
					t.Fatalf("%s: step_data still contains the bank account in plaintext", w.what)
				}
				if strings.Contains(string(stored), realIDCardNo) {
					t.Fatalf("%s: step_data still contains the id card number in plaintext", w.what)
				}

				for key, plaintext := range map[string]string{c.idKey: realIDCardNo, c.bankKey: realBankAccount} {
					got, ok := stepDataValue(t, stored, key)
					if !ok {
						t.Fatalf("%s: key %q disappeared from step_data", w.what, key)
					}
					if got == plaintext {
						t.Fatalf("%s: key %q was bound as plaintext", w.what, key)
					}
					if !enc.LooksEncrypted(got) {
						t.Fatalf("%s: key %q is not ciphertext of this cipher", w.what, key)
					}
					back, err := enc.DecryptFromBase64(got)
					if err != nil || back != plaintext {
						t.Fatalf("%s: key %q does not decrypt back to the original (err=%v)", w.what, key, err)
					}
				}

				// Non-sensitive keys must survive untouched — that is the whole
				// argument for per-key over whole-blob encryption.
				if got, ok := stepDataValue(t, stored, c.untouched); !ok || got != c.untouchedTo {
					t.Fatalf("%s: non-sensitive key %q was altered (got %q, ok=%v)", w.what, c.untouched, got, ok)
				}
			}
		})
	}
}

// TestStepDataEncryptionIsNonDeterministic — two writes of the same document
// must not produce identical ciphertext, otherwise the column becomes a lookup
// table for guessable id card numbers.
func TestStepDataEncryptionIsNonDeterministic(t *testing.T) {
	enc := newTestCipher(t)
	raw := json.RawMessage(`{"idNumber":"` + realIDCardNo + `"}`)

	first, err := encryptStepData(enc, raw)
	if err != nil {
		t.Fatalf("encryptStepData: %v", err)
	}
	second, err := encryptStepData(enc, raw)
	if err != nil {
		t.Fatalf("encryptStepData (2nd): %v", err)
	}
	if string(first) == string(second) {
		t.Fatal("two encryptions of the same step_data produced identical ciphertext (nonce reuse)")
	}
}

// TestStepDataRejectsPlaintextValues is the guard that replaced the pre-#2073
// compatibility branch. Every producer spelling is covered because each one is a
// separate way a value could reach the column unencrypted; before the backfill
// these exact documents read back as-is, which is precisely what made a
// silently-unencrypted value undetectable.
func TestStepDataRejectsPlaintextValues(t *testing.T) {
	enc := newTestCipher(t)
	encryptedID, err := enc.EncryptToBase64(realIDCardNo)
	if err != nil {
		t.Fatalf("EncryptToBase64: %v", err)
	}

	for _, c := range []struct {
		name   string
		stored string
	}{
		{
			name:   "camelCase (app autosave)",
			stored: `{"idNumber":"` + realIDCardNo + `","bankAccount":"` + realBankAccount + `","bankCode":"812"}`,
		},
		{
			name:   "snake_case (app pre-submit save)",
			stored: `{"id_card_no":"` + realIDCardNo + `","bank_account":"` + realBankAccount + `"}`,
		},
		{
			name:   "website form state",
			stored: `{"idCardNo":"` + realIDCardNo + `","bankAccount":"` + realBankAccount + `"}`,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			row := gen.CertificationApplication{
				ID:       7,
				Status:   domain.CertStatusDraft.String(),
				IDCardNo: &encryptedID,
				StepData: []byte(c.stored),
			}
			if _, err := toDomainCertificationApplication(row, enc); err == nil {
				t.Fatal("unencrypted step_data value was accepted — the compatibility layer is back")
			}
		})
	}
}

// TestStepDataPartiallyEncryptedRowIsRejected — a row where one sensitive key is
// encrypted and another is not must not read as if it were fine: the unencrypted
// half is the leak, and the encrypted half would otherwise mask it.
func TestStepDataPartiallyEncryptedRowIsRejected(t *testing.T) {
	enc := newTestCipher(t)
	encryptedID, err := enc.EncryptToBase64(realIDCardNo)
	if err != nil {
		t.Fatalf("EncryptToBase64: %v", err)
	}
	encryptedAccount, err := enc.EncryptToBase64(realBankAccount)
	if err != nil {
		t.Fatalf("EncryptToBase64: %v", err)
	}

	raw := json.RawMessage(`{"idNumber":"` + realIDCardNo + `","bankAccount":"` + encryptedAccount + `"}`)
	row := gen.CertificationApplication{
		ID:       7,
		Status:   domain.CertStatusDraft.String(),
		IDCardNo: &encryptedID,
		StepData: raw,
	}
	if _, err := toDomainCertificationApplication(row, enc); err == nil {
		t.Fatal("a partially-encrypted row was accepted — the unencrypted key went undetected")
	}
}

// TestStepDataRoundTripThroughBothChokePoints walks the real path: domain →
// write params → row → domain.
func TestStepDataRoundTripThroughBothChokePoints(t *testing.T) {
	enc := newTestCipher(t)
	const original = `{"idNumber":"` + realIDCardNo + `","bankAccount":"` + realBankAccount + `","documentReferenceIds":["a","b"],"agreedToTerms":true,"currentStepNote":"步驟三"}`

	app := &domain.CertificationApplication{
		ID:       1,
		Status:   domain.CertStatusDraft,
		IDCardNo: realIDCardNo,
		StepData: json.RawMessage(original),
	}
	params, err := toCreateApplicationParams(app, enc)
	if err != nil {
		t.Fatalf("toCreateApplicationParams: %v", err)
	}

	back, err := toDomainCertificationApplication(gen.CertificationApplication{
		ID:       1,
		Status:   domain.CertStatusDraft.String(),
		IDCardNo: params.IDCardNo,
		StepData: params.StepData,
	}, enc)
	if err != nil {
		t.Fatalf("toDomainCertificationApplication: %v", err)
	}

	var want, got map[string]any
	if err := json.Unmarshal([]byte(original), &want); err != nil {
		t.Fatalf("unmarshal original: %v", err)
	}
	if err := json.Unmarshal(back.StepData, &got); err != nil {
		t.Fatalf("unmarshal round-tripped: %v", err)
	}
	if len(want) != len(got) {
		t.Fatalf("key count changed: wrote %d, read %d", len(want), len(got))
	}
	for k, wv := range want {
		gv, ok := got[k]
		if !ok {
			t.Fatalf("key %q lost in round trip", k)
		}
		if wj, _ := json.Marshal(wv); string(wj) != mustMarshal(t, gv) {
			t.Fatalf("key %q changed value across the round trip", k)
		}
	}
}

func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// TestStepDataPreservesNonSensitiveValuesVerbatim — decoding through
// map[string]any would truncate large integers to float64 and re-escape strings.
// Values we do not touch must come out byte-identical.
func TestStepDataPreservesNonSensitiveValuesVerbatim(t *testing.T) {
	enc := newTestCipher(t)
	// 9007199254740993 = 2^53+1, the first integer float64 cannot represent.
	const raw = `{"idNumber":"` + realIDCardNo + `","applicationRef":9007199254740993,"note":"a<b&c","nested":{"deep":[1,2,{"x":null}]}}`

	stored, err := encryptStepData(enc, json.RawMessage(raw))
	if err != nil {
		t.Fatalf("encryptStepData: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(stored, &fields); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := string(fields["applicationRef"]); got != "9007199254740993" {
		t.Fatalf("large integer was corrupted: got %s", got)
	}
	if got := string(fields["nested"]); got != `{"deep":[1,2,{"x":null}]}` {
		t.Fatalf("nested value was rewritten: got %s", got)
	}
}

// TestStepDataUntouchedShapes — documents that are not objects, are empty, or
// carry the keys with non-string values must pass through unchanged rather than
// erroring. step_data is opaque frontend state; it must never be able to make an
// application unreadable.
func TestStepDataUntouchedShapes(t *testing.T) {
	enc := newTestCipher(t)
	for _, c := range []struct {
		name string
		raw  string
	}{
		{"nil", ""},
		{"empty object", `{}`},
		{"json array", `[1,2,3]`},
		{"malformed", `{not json`},
		{"no sensitive keys", `{"bankCode":"812","agreedToTerms":true}`},
		{"sensitive key is null", `{"idNumber":null}`},
		{"sensitive key is a number", `{"idNumber":123}`},
		{"sensitive key is empty string", `{"idNumber":"","bankAccount":""}`},
	} {
		t.Run(c.name, func(t *testing.T) {
			in := json.RawMessage(c.raw)
			if c.raw == "" {
				in = nil
			}
			out, err := encryptStepData(enc, in)
			if err != nil {
				t.Fatalf("encryptStepData must not fail on %s: %v", c.name, err)
			}
			if string(out) != string(in) {
				t.Fatalf("%s was altered: %q -> %q", c.name, in, out)
			}
			back, err := decryptStepData(enc, in)
			if err != nil {
				t.Fatalf("decryptStepData must not fail on %s: %v", c.name, err)
			}
			if string(back) != string(in) {
				t.Fatalf("%s was altered on read: %q -> %q", c.name, in, back)
			}
		})
	}
}

// TestStepDataRejectsUndecryptableCiphertext — a value that looks like ciphertext
// but will not decrypt (corruption, wrong/rotated key) must fail loudly. Handing
// it back would load garbage into the wizard's form field, and the seller's next
// submit would write that garbage into the real id_card_no / bank_account column.
func TestStepDataRejectsUndecryptableCiphertext(t *testing.T) {
	enc := newTestCipher(t)
	other, err := crypto.New("a-different-master-key")
	if err != nil {
		t.Fatalf("crypto.New: %v", err)
	}
	foreign, err := other.EncryptToBase64(realIDCardNo)
	if err != nil {
		t.Fatalf("EncryptToBase64: %v", err)
	}

	raw := json.RawMessage(`{"idNumber":"` + foreign + `"}`)
	if _, err := decryptStepData(enc, raw); err == nil {
		t.Fatal("expected an error for a value encrypted under a different key, got nil")
	}

	// And it must surface through the read choke point, not be swallowed.
	encryptedID, _ := enc.EncryptToBase64(realIDCardNo)
	_, err = toDomainCertificationApplication(gen.CertificationApplication{
		ID:       7,
		Status:   domain.CertStatusDraft.String(),
		IDCardNo: &encryptedID,
		StepData: raw,
	}, enc)
	if err == nil {
		t.Fatal("toDomainCertificationApplication swallowed an undecryptable step_data value")
	}
}

// TestStepDataErrorDoesNotLeakValue — the error reaches logs and gRPC status
// messages, so it must never carry the stored value.
func TestStepDataErrorDoesNotLeakValue(t *testing.T) {
	enc := newTestCipher(t)
	other, _ := crypto.New("a-different-master-key")
	foreign, _ := other.EncryptToBase64(realIDCardNo)

	_, err := decryptStepData(enc, json.RawMessage(`{"idNumber":"`+foreign+`"}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), foreign) {
		t.Fatal("error message leaks the stored value")
	}
}

// TestStepDataValueAcceptsOnlyOwnCiphertext pins the whole acceptance rule in one
// place: exactly one class of value reads back — ciphertext of this cipher.
// Everything else (plaintext of any length, another key's ciphertext, a truncated
// value) is an error, with no length heuristic left to slip through.
func TestStepDataValueAcceptsOnlyOwnCiphertext(t *testing.T) {
	enc := newTestCipher(t)
	other, err := crypto.New("a-different-master-key")
	if err != nil {
		t.Fatalf("crypto.New: %v", err)
	}
	mine, err := enc.EncryptToBase64(realBankAccount)
	if err != nil {
		t.Fatalf("EncryptToBase64: %v", err)
	}

	for _, plaintext := range []string{"1", realIDCardNo, realBankAccount} {
		foreign, err := other.EncryptToBase64(plaintext)
		if err != nil {
			t.Fatalf("EncryptToBase64: %v", err)
		}
		if _, err := decryptStepDataValue(enc, plaintext); err == nil {
			t.Fatalf("plaintext %d chars was accepted instead of rejected", len(plaintext))
		}
		if _, err := decryptStepDataValue(enc, foreign); err == nil {
			t.Fatalf("wrong-key ciphertext for a %d-char value was accepted instead of rejected", len(plaintext))
		}
	}
	if _, err := decryptStepDataValue(enc, mine[:len(mine)-8]); err == nil {
		t.Fatal("truncated ciphertext was accepted instead of rejected")
	}
	got, err := decryptStepDataValue(enc, mine)
	if err != nil || got != realBankAccount {
		t.Fatalf("own ciphertext did not round-trip: err=%v", err)
	}
}
