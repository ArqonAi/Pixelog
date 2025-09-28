package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	fmt.Println("🧪 Pixelog Compilation Test")
	fmt.Println("============================")

	// Test 1: Check if QR libraries can be imported
	fmt.Println("1. Testing QR library imports...")
	if err := testQRLibraryImports(); err != nil {
		fmt.Printf("❌ QR library test failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ QR libraries import successfully")

	// Test 2: Test if main server compiles
	fmt.Println("\n2. Testing server compilation...")
	if err := testServerCompilation(); err != nil {
		fmt.Printf("❌ Server compilation failed: %v\n", err)
		fmt.Println("This indicates syntax errors or dependency issues in the extraction code")
		os.Exit(1)
	}
	fmt.Println("✅ Server compiles successfully")

	// Test 3: Test if modules are properly resolved
	fmt.Println("\n3. Testing module dependencies...")
	if err := testModuleDependencies(); err != nil {
		fmt.Printf("❌ Module dependency test failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ All module dependencies resolved")

	fmt.Println("\n🎉 COMPILATION TESTS PASSED!")
	fmt.Println("✅ All Go code compiles correctly")
	fmt.Println("✅ QR decoding libraries are available")
	fmt.Println("✅ No syntax errors in extraction implementation")
	fmt.Println("\n⚠️  Note: Runtime testing requires FFmpeg installation")
	fmt.Println("   Install FFmpeg to test actual .pixe extraction functionality")
}

func testQRLibraryImports() error {
	tempFile, err := os.CreateTemp("", "qr_test_*.go")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	testCode := `
package main

import (
	"fmt"
	"github.com/makiuchi-d/gozxing/qrcode"
)

func main() {
	// Test that we can create basic QR structures
	reader := qrcode.NewQRCodeReader()
	if reader == nil {
		panic("Failed to create QR reader")
	}
	
	fmt.Println("QR libraries working correctly")
}
`

	if _, err := tempFile.WriteString(testCode); err != nil {
		return fmt.Errorf("failed to write test code: %w", err)
	}
	tempFile.Close()

	cmd := exec.Command("go", "run", tempFile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("QR library test failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func testServerCompilation() error {
	fmt.Println("  Building server (this tests extraction code compilation)...")
	
	cmd := exec.Command("go", "build", "-o", "/tmp/pixelog-test-server", "./backend/cmd/server")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("server build failed: %w\nBuild output:\n%s", err, string(output))
	}

	// Clean up
	os.Remove("/tmp/pixelog-test-server")
	
	return nil
}

func testModuleDependencies() error {
	cmd := exec.Command("go", "list", "-m", "all")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("module list failed: %w\nOutput: %s", err, string(output))
	}
	
	// Check for key dependencies
	outputStr := string(output)
	requiredModules := []string{
		"github.com/makiuchi-d/gozxing",
		"github.com/gin-gonic/gin",
		"github.com/skip2/go-qrcode",
	}
	
	for _, module := range requiredModules {
		if !contains(outputStr, module) {
			return fmt.Errorf("required module not found: %s", module)
		}
	}
	
	return nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || contains(s[1:], substr)))
}
