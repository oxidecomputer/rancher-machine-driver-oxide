package oxide

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func GetRootDir(t *testing.T) string {
	t.Helper()
	rootDir, err := os.Getwd()
	assert.NilError(t, err)
	return filepath.Dir(filepath.Dir(filepath.Dir(rootDir)))
	//return "."
}

func GetTestdataDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(GetRootDir(t), "./testdata")
}

func GetTestFile(t *testing.T, filename string) string {
	t.Helper()
	return filepath.Join(GetTestdataDir(t), filename)
}

func GetWorkingDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(GetTestdataDir(t), "./working")
}

type MockDriverOptions struct {
	DefaultString      string
	DefaultInt         int
	DefaultStringSlice []string
	DefaultBool        bool
	Values             map[string]interface{}
}

func NewMockDriverOptions(values map[string]interface{}) *MockDriverOptions {
	return &MockDriverOptions{
		DefaultString: "default",
		DefaultInt:    27,
		DefaultStringSlice: []string{
			"yes",
			"please",
		},
		DefaultBool: false,
		Values:      values,
	}
}

func (m *MockDriverOptions) Has(key string) bool {
	_, ok := m.Values[key]
	return ok
}

func (m *MockDriverOptions) String(key string) string {
	if m.Has(key) {
		return m.Values[key].(string)
	}

	return m.DefaultString
}

func (m *MockDriverOptions) StringSlice(key string) []string {
	if m.Has(key) {
		return m.Values[key].([]string)
	}

	return m.DefaultStringSlice
}

func (m *MockDriverOptions) Int(key string) int {
	if m.Has(key) {
		return m.Values[key].(int)
	}

	return m.DefaultInt
}

func (m *MockDriverOptions) Bool(key string) bool {
	if m.Has(key) {
		return m.Values[key].(bool)
	}

	return m.DefaultBool
}
