package rpc

import (
	"slices"
	"strings"
	"testing"

	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	admincontentpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/content/v1alpha1"
	contentpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/content/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
)

func writable(t *testing.T, msg *pb.KYCApplication, paths ...string) []string {
	t.Helper()
	got, err := WritableFields(msg, &fieldmaskpb.FieldMask{Paths: paths})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// 沒帶 mask 是有值的最上層欄位（oneof 的子訊息展開成它裡面可寫的欄位）；明列的欄位不論有沒有值都選到（清掉欄位）；
// OUTPUT_ONLY 與 IDENTIFIER 略過，不回錯。
func TestWritableFieldsFollowTheMask(t *testing.T) {
	app := &pb.KYCApplication{RealName: "王小明", Subject: &pb.KYCApplication_Individual{Individual: &pb.IndividualKYC{IdCardNo: "A123456789"}}}
	if got := writable(t, app); !slices.Equal(got, []string{"real_name", "individual.id_card_no"}) {
		t.Fatalf("沒帶 mask：%v", got)
	}
	if got := writable(t, app, "birth_date", "real_name"); !slices.Equal(got, []string{"birth_date", "real_name"}) {
		t.Fatalf("列出的欄位：%v", got)
	}
	got := writable(t, app, "id", "status", "flow", "missing_fields", "etag", "real_name", "individual.id_card_no", "individual.id_card_no_masked")
	if !slices.Equal(got, []string{"etag", "real_name", "individual.id_card_no"}) {
		t.Fatalf("只讀的欄位要略過：%v", got)
	}
}

// 巢狀路徑只下到 oneof 的子訊息；列到子訊息本身是它全部可寫的欄位；其他子訊息整個換掉，不能再往下。
func TestWritableFieldsNestedPathsOnlyGoIntoOneofBranches(t *testing.T) {
	app := &pb.KYCApplication{}
	if got := writable(t, app, "business"); len(got) != 7 || !slices.Contains(got, "business.company_documents") || slices.Contains(got, "business.company_type") || !slices.Contains(got, "business.company_address") || slices.Contains(got, "business.rep_id_card_no_masked") {
		t.Fatalf("business：%v", got)
	}
	if got := writable(t, app, "individual"); !slices.Equal(got, []string{"individual.id_card_no"}) {
		t.Fatalf("individual：%v", got)
	}
	for _, bad := range []string{"bank_book.asset_id", "birth_date.year", "individual.no_such", "no_such", "business.company_documents.asset_id", "individual.id_card_no.x"} {
		if _, err := WritableFields(app, &fieldmaskpb.FieldMask{Paths: []string{bad}}); err == nil || !strings.Contains(err.Error(), "invalid_argument") {
			t.Errorf("%q 通過了：%v", bad, err)
		}
	}
}

// 可寫的欄位集合完全由 descriptor 決定：「*」等於獨立走一遍 descriptor 算出的集合（最上層的可寫欄位，oneof 只展開設定的那一支），
// 每條路徑都真的存在於 descriptor。
func TestWildcardIsExactlyTheWritableFieldsOfTheDescriptor(t *testing.T) {
	for name, app := range map[string]*pb.KYCApplication{
		"individual": {Subject: &pb.KYCApplication_Individual{Individual: &pb.IndividualKYC{}}},
		"business":   {Subject: &pb.KYCApplication_Business{Business: &pb.BusinessKYC{}}},
		"none":       {},
	} {
		got := writable(t, app, "*")
		want := walkWritable(app.ProtoReflect().Descriptor(), app.ProtoReflect(), "")
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s：\n got=%v\nwant=%v", name, got, want)
		}
		for _, path := range got {
			if !pathExists(app.ProtoReflect().Descriptor(), path) {
				t.Errorf("%s：路徑 %s 不在 descriptor", name, path)
			}
		}
	}
	// 沒設定的那一支不展開。
	if got := writable(t, &pb.KYCApplication{Subject: &pb.KYCApplication_Individual{Individual: &pb.IndividualKYC{}}}, "*"); slices.ContainsFunc(got, func(p string) bool { return strings.HasPrefix(p, "business.") }) {
		t.Fatalf("沒設定的 business 被展開：%v", got)
	}
}

// walkWritable 是獨立於 WritableFields 的實作（測試的預期）。
func walkWritable(desc protoreflect.MessageDescriptor, msg protoreflect.Message, prefix string) []string {
	var out []string
	fields := desc.Fields()
	for i := range fields.Len() {
		fd := fields.Get(i)
		if HasBehavior(fd, annotations.FieldBehavior_OUTPUT_ONLY) || HasBehavior(fd, annotations.FieldBehavior_IDENTIFIER) || HasBehavior(fd, annotations.FieldBehavior_IMMUTABLE) {
			continue
		}
		if o := fd.ContainingOneof(); o != nil && !o.IsSynthetic() && fd.Kind() == protoreflect.MessageKind {
			if msg.Has(fd) {
				out = append(out, walkWritable(fd.Message(), msg.Get(fd).Message(), prefix+string(fd.Name())+".")...)
			}
			continue
		}
		out = append(out, prefix+string(fd.Name()))
	}
	return out
}

// IMMUTABLE：第二種做法拿不到目前值，mask 明列它回 INVALID_ARGUMENT；沒帶 mask 或 "*" 展開時略過。
func TestWritableFieldsRejectAnExplicitImmutableField(t *testing.T) {
	gate := &admincontentpb.AppVersionGate{AppType: contentpb.AppType_APP_TYPE_BUYER, MinimumVersion: "1.2.3"}
	if _, err := WritableFields(gate, &fieldmaskpb.FieldMask{Paths: []string{"app_type", "minimum_version"}}); err == nil || !strings.Contains(err.Error(), "invalid_argument") {
		t.Fatalf("明列 IMMUTABLE 的欄位：err=%v", err)
	}
	for _, paths := range [][]string{nil, {"*"}} {
		got, err := WritableFields(gate, &fieldmaskpb.FieldMask{Paths: paths})
		if err != nil || slices.Contains(got, "app_type") || !slices.Contains(got, "minimum_version") {
			t.Fatalf("paths=%v got=%v err=%v", paths, got, err)
		}
	}
}
