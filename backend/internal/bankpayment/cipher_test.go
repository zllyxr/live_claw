package bankpayment

import "testing"

func TestAccountCipherBindsCiphertextToAccountHash(t *testing.T) {
	cipher, err := NewCipher("bank-payment-test-key-32-characters")
	if err != nil {
		t.Fatal(err)
	}
	secret := AccountSecret{HolderName: "测试收款人", CardNumber: "6222021234567890"}
	ciphertext, err := cipher.EncryptAccount("account-hash", secret)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := cipher.DecryptAccount("account-hash", ciphertext)
	if err != nil || decoded != secret {
		t.Fatalf("decoded=%#v err=%v", decoded, err)
	}
	if _, err = cipher.DecryptAccount("different-account", ciphertext); err == nil {
		t.Fatal("ciphertext could be moved to another bank account")
	}
}

func TestNormalizeAndMaskCardNumber(t *testing.T) {
	card, err := NormalizeCardNumber("6222 0212-3456 7890")
	if err != nil || card != "6222021234567890" {
		t.Fatalf("card=%q err=%v", card, err)
	}
	if masked := MaskCardNumber(card); masked != "**** **** **** 7890" {
		t.Fatalf("masked=%q", masked)
	}
}
