// Command protonumericlint enforces Proto 管理規範 §6: client-exposed (response-side)
// proto message fields must not use int64/int32/double (they lose precision as JSON
// numbers in JS and break the contract — see [[project_hawkr_notification_id_tsid_string]]).
//
// It reads a buf image (FileDescriptorSet) from stdin, walks every message
// transitively reachable from an RPC method's OUTPUT type ("response-exposed"),
// and flags banned numeric scalar fields.
//
// Request-only messages are never reached, so §6.2 exception 1 (Gateway-injected
// identity fields, which live in Requests) does hold for free.
//
// 🔴 Exception 3 does NOT. "Pure service-internal" describes the RPC, not the
// message: a service-internal RPC's *response* type is an OUTPUT type, so the walk
// reaches it like any other. Declaring `int64` there trips this gate even though no
// front end can ever see the value. (2026-08-30: this comment previously claimed the
// opposite and misled an implementer into writing int64 in a new internal RPC.)
// A genuine exception-3 field must go in the allowlist with a reason — and that file
// only shrinks, so in practice: use string on the response side, always.
// §6.2 exception 2 (Response `int32 code`) is honoured by name. Remaining existing
// violations are grandfathered in an allowlist that shrinks as Track 1 fixes land.
//
//	buf build -o - | go run ./cmd/protonumericlint -allowlist scripts/proto/numeric-allowlist.txt
//
// ponytail: map-key types are not checked (no exposed numeric map keys today); add if that changes.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

// bannedKinds are the numeric scalar kinds §6.1 forbids on exposed fields.
// Enums (EnumKind) and bool are intentionally absent. int64/uint64/... lose
// precision past 2^53 as JSON numbers; int32/double are banned by §6.1 too.
var bannedKinds = map[protoreflect.Kind]bool{
	protoreflect.Int32Kind: true, protoreflect.Sint32Kind: true, protoreflect.Sfixed32Kind: true,
	protoreflect.Uint32Kind: true, protoreflect.Fixed32Kind: true,
	protoreflect.Int64Kind: true, protoreflect.Sint64Kind: true, protoreflect.Sfixed64Kind: true,
	protoreflect.Uint64Kind: true, protoreflect.Fixed64Kind: true,
	protoreflect.DoubleKind: true, protoreflect.FloatKind: true,
}

type violation struct {
	key  string
	file string
	line int
	kind string
}

func main() {
	allowPath := flag.String("allowlist", "scripts/proto/numeric-allowlist.txt", "path to the grandfathered-violation allowlist")
	dump := flag.Bool("dump", false, "print all current violations (for seeding the allowlist) and exit 0")
	flag.Parse()

	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		fatal("read stdin (expected `buf build -o -`): %v", err)
	}
	var fdset descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(raw, &fdset); err != nil {
		fatal("unmarshal buf image: %v", err)
	}
	files, err := protodesc.NewFiles(&fdset)
	if err != nil {
		fatal("build descriptors: %v", err)
	}

	vs := findViolations(files)

	if *dump {
		for _, v := range vs {
			fmt.Println(v.key)
		}
		return
	}

	allow := loadAllowlist(*allowPath)
	seen := make(map[string]bool, len(vs))
	var bad []violation
	for _, v := range vs {
		seen[v.key] = true
		if !allow[v.key] {
			bad = append(bad, v)
		}
	}
	var stale []string
	for k := range allow {
		if !seen[k] {
			stale = append(stale, k)
		}
	}
	sort.Strings(stale)

	if len(bad) == 0 && len(stale) == 0 {
		return
	}
	if len(bad) > 0 {
		fmt.Fprintf(os.Stderr, "\nProto 管理規範 §6 violation — exposed (response-side) field must be `string`, not a numeric type:\n")
		for _, v := range bad {
			fmt.Fprintf(os.Stderr, "  %s:%d  %s (%s)\n", v.file, v.line, v.key, v.kind)
		}
		fmt.Fprintf(os.Stderr, "\nFix: change the field to `string` in api/proto and convert at the gRPC boundary (§6.3).\n"+
			"If it is a genuine §6.2 exception, add the fully-qualified name to %s with a reason.\n", *allowPath)
	}
	if len(stale) > 0 {
		fmt.Fprintf(os.Stderr, "\nStale allowlist entries (no longer a violation — delete these lines from %s):\n", *allowPath)
		for _, k := range stale {
			fmt.Fprintf(os.Stderr, "  %s\n", k)
		}
	}
	os.Exit(1)
}

// findViolations walks every message reachable from an RPC output type and
// returns banned numeric fields, sorted by fully-qualified name.
func findViolations(files *protoregistry.Files) []violation {
	visited := make(map[protoreflect.FullName]bool)
	var queue []protoreflect.MessageDescriptor

	enqueue := func(m protoreflect.MessageDescriptor) {
		if m == nil || visited[m.FullName()] || m.IsMapEntry() {
			return
		}
		if strings.HasPrefix(string(m.ParentFile().Package()), "google.") {
			return // well-known types (Timestamp.seconds etc.) are not our contract
		}
		visited[m.FullName()] = true
		queue = append(queue, m)
	}

	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		svcs := fd.Services()
		for i := 0; i < svcs.Len(); i++ {
			methods := svcs.Get(i).Methods()
			for j := 0; j < methods.Len(); j++ {
				enqueue(methods.Get(j).Output())
			}
		}
		return true
	})

	var out []violation
	for len(queue) > 0 {
		m := queue[0]
		queue = queue[1:]
		fields := m.Fields()
		for i := 0; i < fields.Len(); i++ {
			f := fields.Get(i)
			if f.IsMap() {
				if mv := f.MapValue(); mv.Message() != nil {
					enqueue(mv.Message())
				} else if bannedKinds[mv.Kind()] {
					out = append(out, mkViolation(m, f, mv.Kind()))
				}
				continue
			}
			if f.Message() != nil {
				enqueue(f.Message())
				continue
			}
			if bannedKinds[f.Kind()] && string(f.Name()) != "code" { // §6.2 #2
				out = append(out, mkViolation(m, f, f.Kind()))
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key < out[j].key })
	return out
}

func mkViolation(m protoreflect.MessageDescriptor, f protoreflect.FieldDescriptor, k protoreflect.Kind) violation {
	v := violation{key: string(f.FullName()), file: m.ParentFile().Path(), kind: k.String()}
	if loc := m.ParentFile().SourceLocations().ByDescriptor(f); len(loc.Path) > 0 {
		v.line = loc.StartLine + 1 // StartLine is 0-based
	}
	return v
}

func loadAllowlist(path string) map[string]bool {
	allow := make(map[string]bool)
	file, err := os.Open(path)
	if err != nil {
		return allow // first run / no allowlist yet
	}
	defer func() { _ = file.Close() }()
	sc := bufio.NewScanner(file)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		allow[line] = true
	}
	return allow
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "protonumericlint: "+format+"\n", args...)
	os.Exit(2)
}
