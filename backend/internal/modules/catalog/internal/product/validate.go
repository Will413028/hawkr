package product

import (
	"errors"
	"fmt"
	"slices"
	"unicode/utf8"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/description"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/hashtag"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

type prepared struct {
	title      string
	categoryID uuid.UUID
	desc       []model.Block
	gallery    []model.Image
	axes       []model.Axis
	skus       []model.SkuInput
	tags       []hashtag.Tag
	lowStock   *int32
	confirmed map[uuid.UUID]bool
}

func prepare(in model.ProductInput) (prepared, error) {
	var p prepared
	p.title = trimtext.Trim(in.Title)
	if p.title == "" {
		return p, model.Invalid("Title", model.ReasonRequired)
	}
	if utf8.RuneCountInString(p.title) > model.MaxProductInputTitleRunes {
		return p, model.Invalid("Title", model.ReasonTooLong)
	}
	if in.CategoryID == uuid.Nil() {
		return p, model.Invalid("CategoryID", model.ReasonRequired)
	}
	p.categoryID = in.CategoryID
	if t := in.LowStockThreshold; t != nil {
		if *t < 0 || *t > model.MaxLowStockThreshold {
			return p, model.Invalid("LowStockThreshold", model.ReasonOutOfRange)
		}
		p.lowStock = t
	}

	var err error
	if p.desc, err = description.Normalize("Description", in.Description); err != nil {
		return p, err
	}
	if p.gallery, err = normalizeGallery(in.Gallery); err != nil {
		return p, err
	}
	if p.axes, err = normalizeAxes(in.Axes); err != nil {
		return p, err
	}
	if p.skus, err = normalizeSkus(p.axes, in.Skus); err != nil {
		return p, err
	}
	if p.tags, err = hashtag.RequireSet(in.Hashtags); err != nil {
		return p, hashtagInvalid(err)
	}
	return p, checkImageRefsUnique(p.gallery, p.desc)
}

func hashtagInvalid(err error) error {
	reason := model.ReasonInvalid
	switch {
	case errors.Is(err, hashtag.ErrEmpty):
		reason = model.ReasonRequired
	case errors.Is(err, hashtag.ErrLength):
		reason = model.ReasonTooLong
	case errors.Is(err, hashtag.ErrTooMany):
		reason = model.ReasonTooMany
	}
	e := model.Invalid("Hashtags", reason)
	e.Cause = err
	return e
}

func normalizeGallery(in []model.Image) ([]model.Image, error) {
	if len(in) > model.MaxProductInputGallery {
		return nil, model.Invalid("Gallery", model.ReasonTooMany)
	}
	out := make([]model.Image, len(in))
	for i, img := range in {
		n, err := description.NormalizeImage(fmt.Sprintf("Gallery[%d]", i), img)
		if err != nil {
			return nil, err
		}
		out[i] = n
	}
	return out, nil
}

func checkImageRefsUnique(gallery []model.Image, desc []model.Block) error {
	tokens, assets := map[string]bool{}, map[uuid.UUID]bool{}
	check := func(field string, img model.Image) error {
		if img.Token != "" {
			if tokens[img.Token] {
				return model.Invalid(field, model.ReasonDuplicate)
			}
			tokens[img.Token] = true
		} else {
			if assets[img.AssetID] {
				return model.Invalid(field, model.ReasonDuplicate)
			}
			assets[img.AssetID] = true
		}
		return nil
	}
	for i, img := range gallery {
		if err := check(fmt.Sprintf("Gallery[%d]", i), img); err != nil {
			return err
		}
	}
	for i, ref := range description.ImageRefs(desc) {
		if err := check(fmt.Sprintf("Description.Image[%d]", i), *ref); err != nil {
			return err
		}
	}
	return nil
}

func normalizeAxes(in []model.Axis) ([]model.Axis, error) {
	if len(in) > model.MaxProductInputAxes {
		return nil, model.Invalid("Axes", model.ReasonTooMany)
	}
	out := make([]model.Axis, len(in))
	names := map[string]bool{}
	for i, a := range in {
		f := fmt.Sprintf("Axes[%d]", i)
		name := trimtext.Trim(a.Name)
		if name == "" {
			return nil, model.Invalid(f+".Name", model.ReasonRequired)
		}
		if utf8.RuneCountInString(name) > model.MaxOptionAxisInputNameRunes {
			return nil, model.Invalid(f+".Name", model.ReasonTooLong)
		}
		if names[name] {
			return nil, model.Invalid(f+".Name", model.ReasonDuplicate)
		}
		names[name] = true
		if len(a.Values) == 0 {
			return nil, model.Invalid(f+".Values", model.ReasonRequired)
		}
		if len(a.Values) > model.MaxOptionAxisInputValues {
			return nil, model.Invalid(f+".Values", model.ReasonTooMany)
		}
		values := make([]string, len(a.Values))
		for j, v := range a.Values {
			v = trimtext.Trim(v)
			vf := fmt.Sprintf("%s.Values[%d]", f, j)
			if v == "" {
				return nil, model.Invalid(vf, model.ReasonRequired)
			}
			if utf8.RuneCountInString(v) > model.MaxOptionAxisInputValuesRunes {
				return nil, model.Invalid(vf, model.ReasonTooLong)
			}
			if slices.Contains(values[:j], v) {
				return nil, model.Invalid(vf, model.ReasonDuplicate)
			}
			values[j] = v
		}
		out[i] = model.Axis{Name: name, Values: values}
	}
	return out, nil
}

func normalizeSkus(axes []model.Axis, in []model.SkuInput) ([]model.SkuInput, error) {
	if len(in) == 0 {
		return nil, model.Invalid("Skus", model.ReasonRequired)
	}
	if len(in) > model.MaxProductInputSkus {
		return nil, model.Invalid("Skus", model.ReasonTooMany)
	}
	if len(axes) == 0 && len(in) != 1 {
		return nil, model.Invalid("Skus", model.ReasonTooMany)
	}
	out := make([]model.SkuInput, len(in))
	seen := map[[2]string]bool{}
	for i, s := range in {
		f := fmt.Sprintf("Skus[%d]", i)
		s.Option1, s.Option2 = trimtext.Trim(s.Option1), trimtext.Trim(s.Option2)
		for k, opt := range [2]*string{&s.Option1, &s.Option2} {
			optField := fmt.Sprintf("%s.Option%d", f, k+1)
			switch {
			case k >= len(axes):
				if *opt != "" {
					return nil, model.Invalid(optField, model.ReasonUnknown)
				}
			case *opt == "":
				return nil, model.Invalid(optField, model.ReasonRequired)
			case !slices.Contains(axes[k].Values, *opt):
				return nil, model.Invalid(optField, model.ReasonUnknown)
			}
		}
		key := [2]string{s.Option1, s.Option2}
		if seen[key] {
			return nil, model.Invalid(f, model.ReasonDuplicate)
		}
		seen[key] = true
		if s.Price <= 0 {
			return nil, model.Invalid(f+".Price", model.ReasonOutOfRange)
		}
		if s.CompareAtPrice != nil && *s.CompareAtPrice <= s.Price {
			return nil, model.Invalid(f+".CompareAtPrice", model.ReasonOutOfRange)
		}
		if s.StockQty < 0 {
			return nil, model.Invalid(f+".StockQty", model.ReasonOutOfRange)
		}
		out[i] = s
	}
	return out, nil
}
