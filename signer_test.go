package forest_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"testing"

	forest "git.sr.ht/~whereswaldon/forest-go"
	"git.sr.ht/~whereswaldon/forest-go/testkeys"
)

// ensureGPGInstalled will cause the calling test to be skipped if GPG
// isn't available on the system. It returns the path to the gpg executable
// if it is available
func ensureGPGInstalled(t *testing.T) string {
	gpg, err := forest.FindGPG()
	if err != nil {
		t.Skip("GPG not available", err)
		t.SkipNow()
	}
	return gpg
}

const testUsername = "Arbor-Dev-Untrusted-Test-01"
const testData = "testdata"

// TestGPGSigner creates a new GPG key in a temporary directory and signs some data.
func TestGPGSigner(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping expensive GPG test in short mode")
	}
	signer, cleanup := getGPGSignerOrFail(t)
	defer cleanup()
	// sign some data
	signature, err := signer.Sign([]byte(testData))
	if err != nil {
		t.Errorf("Failed sign data: %v", err)
	} else if len(signature) < 1 {
		t.Errorf("Signing produced empty signature")
	}
}

func getGPGSignerOrFail(t *testing.T) (forest.Signer, func()) {
	gpgExec := ensureGPGInstalled(t)

	// generate PGP key to use
	tempdir, err := ioutil.TempDir("", "arborchat-test")
	if err != nil {
		t.Errorf("Failed to create temporary GNUPG home: %v", err)
	}

	tempkey, err := ioutil.TempFile(tempdir, "testPrivKey.key")
	if _, err = tempkey.Write([]byte(testkeys.PrivKey1)); err != nil {
		t.Errorf("Failed to create temporary gpg key: %v", err)
	}

	cleanup := func() { os.RemoveAll(tempdir) }
	gpg2 := exec.Command(gpgExec, "--yes", "--batch", "--pinentry-mode", "loopback", "--import", tempkey.Name())
	gpg2.Env = []string{"GNUPGHOME=" + tempdir}
	stderr, _ := gpg2.StderrPipe()
	if err := gpg2.Run(); err != nil {
		data, _ := ioutil.ReadAll(stderr)
		t.Log(data)
		t.Errorf("Error generating key: %v", err)
		cleanup()
	}
	// build signer
	signer, err := forest.NewGPGSigner(testUsername)
	if err != nil {
		t.Fatalf("Failed to construct signer with valid username: %v", err)
		cleanup()
	}
	signer.Rewriter = func(gpg2 *exec.Cmd) error {
		gpg2.Args = append(append(gpg2.Args[:1], "--yes", "--batch", "--pinentry-mode", "loopback", "--passphrase", testkeys.TestKeyPassphrase), gpg2.Args[1:]...)
		gpg2.Env = []string{"GNUPGHOME=" + tempdir}
		gpg2.Stderr = os.Stderr
		return nil
	}
	return signer, cleanup
}

func TestGPGSignerAsIdentity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping expensive GPG test in short mode")
	}
	signer, cleanup := getGPGSignerOrFail(t)
	defer cleanup()
	identity, err := forest.NewIdentity(signer, "test name", "")
	if err != nil {
		t.Fatal("Failed to create Identity with valid parameters", err)
	}
	if correct, err := forest.ValidateID(identity, *identity.ID()); err != nil || !correct {
		t.Error("ID validation failed on unmodified node", err)
	}
	if correct, err := forest.ValidateSignature(identity, identity); err != nil || !correct {
		t.Error("Signature validation failed on unmodified node", err)
	}
}
