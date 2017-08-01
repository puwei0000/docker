package volume

import (
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/mount"
)

type parseMountRawTestSet struct {
	valid   []string
	invalid map[string]string
}

func TestConvertTmpfsOptions(t *testing.T) {
	type testCase struct {
		opt                  mount.TmpfsOptions
		readOnly             bool
		expectedSubstrings   []string
		unexpectedSubstrings []string
	}
	cases := []testCase{
		{
			opt:                  mount.TmpfsOptions{SizeBytes: 1024 * 1024, Mode: 0700},
			readOnly:             false,
			expectedSubstrings:   []string{"size=1m", "mode=700"},
			unexpectedSubstrings: []string{"ro"},
		},
		{
			opt:                  mount.TmpfsOptions{},
			readOnly:             true,
			expectedSubstrings:   []string{"ro"},
			unexpectedSubstrings: []string{},
		},
	}
	p := &linuxParser{}
	for _, c := range cases {
		data, err := p.ConvertTmpfsOptions(&c.opt, c.readOnly)
		if err != nil {
			t.Fatalf("could not convert %+v (readOnly: %v) to string: %v",
				c.opt, c.readOnly, err)
		}
		t.Logf("data=%q", data)
		for _, s := range c.expectedSubstrings {
			if !strings.Contains(data, s) {
				t.Fatalf("expected substring: %s, got %v (case=%+v)", s, data, c)
			}
		}
		for _, s := range c.unexpectedSubstrings {
			if strings.Contains(data, s) {
				t.Fatalf("unexpected substring: %s, got %v (case=%+v)", s, data, c)
			}
		}
	}
}

type mockFiProvider struct{}

func (mockFiProvider) fileInfo(path string) (exists, isDir bool, err error) {
	dirs := map[string]struct{}{
		`c:\`:                    struct{}{},
		`c:\windows\`:            struct{}{},
		`c:\windows`:             struct{}{},
		`c:\program files`:       struct{}{},
		`c:\Windows`:             struct{}{},
		`c:\Program Files (x86)`: struct{}{},
	}
	files := map[string]struct{}{
		`c:\windows\system32\ntdll.dll`: struct{}{},
	}
	if _, ok := dirs[path]; ok {
		return true, true, nil
	}
	if _, ok := files[path]; ok {
		return true, false, nil
	}
	return false, false, nil
}

func TestParseMountRaw(t *testing.T) {

	previousProvider := currentFileInfoProvider
	defer func() { currentFileInfoProvider = previousProvider }()
	currentFileInfoProvider = mockFiProvider{}
	windowsSet := parseMountRawTestSet{
		valid: []string{
			`d:\`,
			`d:`,
			`d:\path`,
			`d:\path with space`,
			`c:\:d:\`,
			`c:\windows\:d:`,
			`c:\windows:d:\s p a c e`,
			`c:\windows:d:\s p a c e:RW`,
			`c:\program files:d:\s p a c e i n h o s t d i r`,
			`0123456789name:d:`,
			`MiXeDcAsEnAmE:d:`,
			`name:D:`,
			`name:D::rW`,
			`name:D::RW`,
			`name:D::RO`,
			`c:/:d:/forward/slashes/are/good/too`,
			`c:/:d:/including with/spaces:ro`,
			`c:\Windows`,             // With capital
			`c:\Program Files (x86)`, // With capitals and brackets
		},
		invalid: map[string]string{
			``:                                 "invalid volume specification: ",
			`.`:                                "invalid volume specification: ",
			`..\`:                              "invalid volume specification: ",
			`c:\:..\`:                          "invalid volume specification: ",
			`c:\:d:\:xyzzy`:                    "invalid volume specification: ",
			`c:`:                               "cannot be `c:`",
			`c:\`:                              "cannot be `c:`",
			`c:\notexist:d:`:                   `source path does not exist`,
			`c:\windows\system32\ntdll.dll:d:`: `source path must be a directory`,
			`name<:d:`:                         `invalid volume specification`,
			`name>:d:`:                         `invalid volume specification`,
			`name::d:`:                         `invalid volume specification`,
			`name":d:`:                         `invalid volume specification`,
			`name\:d:`:                         `invalid volume specification`,
			`name*:d:`:                         `invalid volume specification`,
			`name|:d:`:                         `invalid volume specification`,
			`name?:d:`:                         `invalid volume specification`,
			`name/:d:`:                         `invalid volume specification`,
			`d:\pathandmode:rw`:                `invalid volume specification`,
			`d:\pathandmode:ro`:                `invalid volume specification`,
			`con:d:`:                           `cannot be a reserved word for Windows filenames`,
			`PRN:d:`:                           `cannot be a reserved word for Windows filenames`,
			`aUx:d:`:                           `cannot be a reserved word for Windows filenames`,
			`nul:d:`:                           `cannot be a reserved word for Windows filenames`,
			`com1:d:`:                          `cannot be a reserved word for Windows filenames`,
			`com2:d:`:                          `cannot be a reserved word for Windows filenames`,
			`com3:d:`:                          `cannot be a reserved word for Windows filenames`,
			`com4:d:`:                          `cannot be a reserved word for Windows filenames`,
			`com5:d:`:                          `cannot be a reserved word for Windows filenames`,
			`com6:d:`:                          `cannot be a reserved word for Windows filenames`,
			`com7:d:`:                          `cannot be a reserved word for Windows filenames`,
			`com8:d:`:                          `cannot be a reserved word for Windows filenames`,
			`com9:d:`:                          `cannot be a reserved word for Windows filenames`,
			`lpt1:d:`:                          `cannot be a reserved word for Windows filenames`,
			`lpt2:d:`:                          `cannot be a reserved word for Windows filenames`,
			`lpt3:d:`:                          `cannot be a reserved word for Windows filenames`,
			`lpt4:d:`:                          `cannot be a reserved word for Windows filenames`,
			`lpt5:d:`:                          `cannot be a reserved word for Windows filenames`,
			`lpt6:d:`:                          `cannot be a reserved word for Windows filenames`,
			`lpt7:d:`:                          `cannot be a reserved word for Windows filenames`,
			`lpt8:d:`:                          `cannot be a reserved word for Windows filenames`,
			`lpt9:d:`:                          `cannot be a reserved word for Windows filenames`,
			`c:\windows\system32\ntdll.dll`:    `Only directories can be mapped on this platform`,
		},
	}
	lcowSet := parseMountRawTestSet{
		valid: []string{
			`/foo`,
			`/foo/`,
			`/foo bar`,
			`c:\:/foo`,
			`c:\windows\:/foo`,
			`c:\windows:/s p a c e`,
			`c:\windows:/s p a c e:RW`,
			`c:\program files:/s p a c e i n h o s t d i r`,
			`0123456789name:/foo`,
			`MiXeDcAsEnAmE:/foo`,
			`name:/foo`,
			`name:/foo:rW`,
			`name:/foo:RW`,
			`name:/foo:RO`,
			`c:/:/forward/slashes/are/good/too`,
			`c:/:/including with/spaces:ro`,
			`/Program Files (x86)`, // With capitals and brackets
		},
		invalid: map[string]string{
			``:                                   "invalid volume specification: ",
			`.`:                                  "invalid volume specification: ",
			`c:`:                                 "invalid volume specification: ",
			`c:\`:                                "invalid volume specification: ",
			`../`:                                "invalid volume specification: ",
			`c:\:../`:                            "invalid volume specification: ",
			`c:\:/foo:xyzzy`:                     "invalid volume specification: ",
			`/`:                                  "destination can't be '/'",
			`/..`:                                "destination can't be '/'",
			`c:\notexist:/foo`:                   `source path does not exist`,
			`c:\windows\system32\ntdll.dll:/foo`: `source path must be a directory`,
			`name<:/foo`:                         `invalid volume specification`,
			`name>:/foo`:                         `invalid volume specification`,
			`name::/foo`:                         `invalid volume specification`,
			`name":/foo`:                         `invalid volume specification`,
			`name\:/foo`:                         `invalid volume specification`,
			`name*:/foo`:                         `invalid volume specification`,
			`name|:/foo`:                         `invalid volume specification`,
			`name?:/foo`:                         `invalid volume specification`,
			`name/:/foo`:                         `invalid volume specification`,
			`/foo:rw`:                            `invalid volume specification`,
			`/foo:ro`:                            `invalid volume specification`,
			`con:/foo`:                           `cannot be a reserved word for Windows filenames`,
			`PRN:/foo`:                           `cannot be a reserved word for Windows filenames`,
			`aUx:/foo`:                           `cannot be a reserved word for Windows filenames`,
			`nul:/foo`:                           `cannot be a reserved word for Windows filenames`,
			`com1:/foo`:                          `cannot be a reserved word for Windows filenames`,
			`com2:/foo`:                          `cannot be a reserved word for Windows filenames`,
			`com3:/foo`:                          `cannot be a reserved word for Windows filenames`,
			`com4:/foo`:                          `cannot be a reserved word for Windows filenames`,
			`com5:/foo`:                          `cannot be a reserved word for Windows filenames`,
			`com6:/foo`:                          `cannot be a reserved word for Windows filenames`,
			`com7:/foo`:                          `cannot be a reserved word for Windows filenames`,
			`com8:/foo`:                          `cannot be a reserved word for Windows filenames`,
			`com9:/foo`:                          `cannot be a reserved word for Windows filenames`,
			`lpt1:/foo`:                          `cannot be a reserved word for Windows filenames`,
			`lpt2:/foo`:                          `cannot be a reserved word for Windows filenames`,
			`lpt3:/foo`:                          `cannot be a reserved word for Windows filenames`,
			`lpt4:/foo`:                          `cannot be a reserved word for Windows filenames`,
			`lpt5:/foo`:                          `cannot be a reserved word for Windows filenames`,
			`lpt6:/foo`:                          `cannot be a reserved word for Windows filenames`,
			`lpt7:/foo`:                          `cannot be a reserved word for Windows filenames`,
			`lpt8:/foo`:                          `cannot be a reserved word for Windows filenames`,
			`lpt9:/foo`:                          `cannot be a reserved word for Windows filenames`,
		},
	}
	linuxSet := parseMountRawTestSet{
		valid: []string{
			"/home",
			"/home:/home",
			"/home:/something/else",
			"/with space",
			"/home:/with space",
			"relative:/absolute-path",
			"hostPath:/containerPath:ro",
			"/hostPath:/containerPath:rw",
			"/rw:/ro",
			"/hostPath:/containerPath:shared",
			"/hostPath:/containerPath:rshared",
			"/hostPath:/containerPath:slave",
			"/hostPath:/containerPath:rslave",
			"/hostPath:/containerPath:private",
			"/hostPath:/containerPath:rprivate",
			"/hostPath:/containerPath:ro,shared",
			"/hostPath:/containerPath:ro,slave",
			"/hostPath:/containerPath:ro,private",
			"/hostPath:/containerPath:ro,z,shared",
			"/hostPath:/containerPath:ro,Z,slave",
			"/hostPath:/containerPath:Z,ro,slave",
			"/hostPath:/containerPath:slave,Z,ro",
			"/hostPath:/containerPath:Z,slave,ro",
			"/hostPath:/containerPath:slave,ro,Z",
			"/hostPath:/containerPath:rslave,ro,Z",
			"/hostPath:/containerPath:ro,rshared,Z",
			"/hostPath:/containerPath:ro,Z,rprivate",
		},
		invalid: map[string]string{
			"":                                "invalid volume specification",
			"./":                              "mount path must be absolute",
			"../":                             "mount path must be absolute",
			"/:../":                           "mount path must be absolute",
			"/:path":                          "mount path must be absolute",
			":":                               "invalid volume specification",
			"/tmp:":                           "invalid volume specification",
			":test":                           "invalid volume specification",
			":/test":                          "invalid volume specification",
			"tmp:":                            "invalid volume specification",
			":test:":                          "invalid volume specification",
			"::":                              "invalid volume specification",
			":::":                             "invalid volume specification",
			"/tmp:::":                         "invalid volume specification",
			":/tmp::":                         "invalid volume specification",
			"/path:rw":                        "invalid volume specification",
			"/path:ro":                        "invalid volume specification",
			"/rw:rw":                          "invalid volume specification",
			"path:ro":                         "invalid volume specification",
			"/path:/path:sw":                  `invalid mode`,
			"/path:/path:rwz":                 `invalid mode`,
			"/path:/path:ro,rshared,rslave":   `invalid mode`,
			"/path:/path:ro,z,rshared,rslave": `invalid mode`,
			"/path:shared":                    "invalid volume specification",
			"/path:slave":                     "invalid volume specification",
			"/path:private":                   "invalid volume specification",
			"name:/absolute-path:shared":      "invalid volume specification",
			"name:/absolute-path:rshared":     "invalid volume specification",
			"name:/absolute-path:slave":       "invalid volume specification",
			"name:/absolute-path:rslave":      "invalid volume specification",
			"name:/absolute-path:private":     "invalid volume specification",
			"name:/absolute-path:rprivate":    "invalid volume specification",
		},
	}

	linParser := &linuxParser{}
	winParser := &windowsParser{}
	lcowParser := &lcowParser{}
	tester := func(parser Parser, set parseMountRawTestSet) {

		for _, path := range set.valid {

			if _, err := parser.ParseMountRaw(path, "local"); err != nil {
				t.Fatalf("ParseMountRaw(`%q`) should succeed: error %q", path, err)
			}
		}

		for path, expectedError := range set.invalid {
			if mp, err := parser.ParseMountRaw(path, "local"); err == nil {
				t.Fatalf("ParseMountRaw(`%q`) should have failed validation. Err '%v' - MP: %v", path, err, mp)
			} else {
				if !strings.Contains(err.Error(), expectedError) {
					t.Fatalf("ParseMountRaw(`%q`) error should contain %q, got %v", path, expectedError, err.Error())
				}
			}
		}
	}
	tester(linParser, linuxSet)
	tester(winParser, windowsSet)
	tester(lcowParser, lcowSet)

}

// testParseMountRaw is a structure used by TestParseMountRawSplit for
// specifying test cases for the ParseMountRaw() function.
type testParseMountRaw struct {
	bind      string
	driver    string
	expDest   string
	expSource string
	expName   string
	expDriver string
	expRW     bool
	fail      bool
}

func TestParseMountRawSplit(t *testing.T) {
	previousProvider := currentFileInfoProvider
	defer func() { currentFileInfoProvider = previousProvider }()
	currentFileInfoProvider = mockFiProvider{}
	windowsCases := []testParseMountRaw{
		{`c:\:d:`, "local", `d:`, `c:\`, ``, "", true, false},
		{`c:\:d:\`, "local", `d:\`, `c:\`, ``, "", true, false},
		{`c:\:d:\:ro`, "local", `d:\`, `c:\`, ``, "", false, false},
		{`c:\:d:\:rw`, "local", `d:\`, `c:\`, ``, "", true, false},
		{`c:\:d:\:foo`, "local", `d:\`, `c:\`, ``, "", false, true},
		{`name:d::rw`, "local", `d:`, ``, `name`, "local", true, false},
		{`name:d:`, "local", `d:`, ``, `name`, "local", true, false},
		{`name:d::ro`, "local", `d:`, ``, `name`, "local", false, false},
		{`name:c:`, "", ``, ``, ``, "", true, true},
		{`driver/name:c:`, "", ``, ``, ``, "", true, true},
	}
	lcowCases := []testParseMountRaw{
		{`c:\:/foo`, "local", `/foo`, `c:\`, ``, "", true, false},
		{`c:\:/foo:ro`, "local", `/foo`, `c:\`, ``, "", false, false},
		{`c:\:/foo:rw`, "local", `/foo`, `c:\`, ``, "", true, false},
		{`c:\:/foo:foo`, "local", `/foo`, `c:\`, ``, "", false, true},
		{`name:/foo:rw`, "local", `/foo`, ``, `name`, "local", true, false},
		{`name:/foo`, "local", `/foo`, ``, `name`, "local", true, false},
		{`name:/foo:ro`, "local", `/foo`, ``, `name`, "local", false, false},
		{`name:/`, "", ``, ``, ``, "", true, true},
		{`driver/name:/`, "", ``, ``, ``, "", true, true},
	}
	linuxCases := []testParseMountRaw{
		{"/tmp:/tmp1", "", "/tmp1", "/tmp", "", "", true, false},
		{"/tmp:/tmp2:ro", "", "/tmp2", "/tmp", "", "", false, false},
		{"/tmp:/tmp3:rw", "", "/tmp3", "/tmp", "", "", true, false},
		{"/tmp:/tmp4:foo", "", "", "", "", "", false, true},
		{"name:/named1", "", "/named1", "", "name", "", true, false},
		{"name:/named2", "external", "/named2", "", "name", "external", true, false},
		{"name:/named3:ro", "local", "/named3", "", "name", "local", false, false},
		{"local/name:/tmp:rw", "", "/tmp", "", "local/name", "", true, false},
		{"/tmp:tmp", "", "", "", "", "", true, true},
	}
	linParser := &linuxParser{}
	winParser := &windowsParser{}
	lcowParser := &lcowParser{}
	tester := func(parser Parser, cases []testParseMountRaw) {
		for i, c := range cases {
			t.Logf("case %d", i)
			m, err := parser.ParseMountRaw(c.bind, c.driver)
			if c.fail {
				if err == nil {
					t.Fatalf("Expected error, was nil, for spec %s\n", c.bind)
				}
				continue
			}

			if m == nil || err != nil {
				t.Fatalf("ParseMountRaw failed for spec '%s', driver '%s', error '%v'", c.bind, c.driver, err.Error())
				continue
			}

			if m.Destination != c.expDest {
				t.Fatalf("Expected destination '%s, was %s', for spec '%s'", c.expDest, m.Destination, c.bind)
			}

			if m.Source != c.expSource {
				t.Fatalf("Expected source '%s', was '%s', for spec '%s'", c.expSource, m.Source, c.bind)
			}

			if m.Name != c.expName {
				t.Fatalf("Expected name '%s', was '%s' for spec '%s'", c.expName, m.Name, c.bind)
			}

			if m.Driver != c.expDriver {
				t.Fatalf("Expected driver '%s', was '%s', for spec '%s'", c.expDriver, m.Driver, c.bind)
			}

			if m.RW != c.expRW {
				t.Fatalf("Expected RW '%v', was '%v' for spec '%s'", c.expRW, m.RW, c.bind)
			}
		}
	}

	tester(linParser, linuxCases)
	tester(winParser, windowsCases)
	tester(lcowParser, lcowCases)
}

func TestParseMountSpec(t *testing.T) {
	type c struct {
		input    mount.Mount
		expected MountPoint
	}
	testDir, err := ioutil.TempDir("", "test-mount-config")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(testDir)
	parser := NewParser(runtime.GOOS)
	cases := []c{
		{mount.Mount{Type: mount.TypeBind, Source: testDir, Target: testDestinationPath, ReadOnly: true}, MountPoint{Type: mount.TypeBind, Source: testDir, Destination: testDestinationPath, Propagation: parser.DefaultPropagationMode()}},
		{mount.Mount{Type: mount.TypeBind, Source: testDir, Target: testDestinationPath}, MountPoint{Type: mount.TypeBind, Source: testDir, Destination: testDestinationPath, RW: true, Propagation: parser.DefaultPropagationMode()}},
		{mount.Mount{Type: mount.TypeBind, Source: testDir + string(os.PathSeparator), Target: testDestinationPath, ReadOnly: true}, MountPoint{Type: mount.TypeBind, Source: testDir, Destination: testDestinationPath, Propagation: parser.DefaultPropagationMode()}},
		{mount.Mount{Type: mount.TypeBind, Source: testDir, Target: testDestinationPath + string(os.PathSeparator), ReadOnly: true}, MountPoint{Type: mount.TypeBind, Source: testDir, Destination: testDestinationPath, Propagation: parser.DefaultPropagationMode()}},
		{mount.Mount{Type: mount.TypeVolume, Target: testDestinationPath}, MountPoint{Type: mount.TypeVolume, Destination: testDestinationPath, RW: true, CopyData: parser.DefaultCopyMode()}},
		{mount.Mount{Type: mount.TypeVolume, Target: testDestinationPath + string(os.PathSeparator)}, MountPoint{Type: mount.TypeVolume, Destination: testDestinationPath, RW: true, CopyData: parser.DefaultCopyMode()}},
	}

	for i, c := range cases {
		t.Logf("case %d", i)
		mp, err := parser.ParseMountSpec(c.input)
		if err != nil {
			t.Fatal(err)
		}

		if c.expected.Type != mp.Type {
			t.Fatalf("Expected mount types to match. Expected: '%s', Actual: '%s'", c.expected.Type, mp.Type)
		}
		if c.expected.Destination != mp.Destination {
			t.Fatalf("Expected mount destination to match. Expected: '%s', Actual: '%s'", c.expected.Destination, mp.Destination)
		}
		if c.expected.Source != mp.Source {
			t.Fatalf("Expected mount source to match. Expected: '%s', Actual: '%s'", c.expected.Source, mp.Source)
		}
		if c.expected.RW != mp.RW {
			t.Fatalf("Expected mount writable to match. Expected: '%v', Actual: '%v'", c.expected.RW, mp.RW)
		}
		if c.expected.Propagation != mp.Propagation {
			t.Fatalf("Expected mount propagation to match. Expected: '%v', Actual: '%s'", c.expected.Propagation, mp.Propagation)
		}
		if c.expected.Driver != mp.Driver {
			t.Fatalf("Expected mount driver to match. Expected: '%v', Actual: '%s'", c.expected.Driver, mp.Driver)
		}
		if c.expected.CopyData != mp.CopyData {
			t.Fatalf("Expected mount copy data to match. Expected: '%v', Actual: '%v'", c.expected.CopyData, mp.CopyData)
		}
	}
}
