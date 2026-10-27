package domain

type CategoryOrigin struct {
	declared     bool
	id           string
	absentReason string
}

func CategoryFromSelection(sel *CropSelection) CategoryOrigin {
	id, _ := sel.ConfirmedCategory()
	return CategoryOrigin{declared: true, id: id}
}

func CategoryAbsent(reason string) CategoryOrigin {
	return CategoryOrigin{declared: true, absentReason: reason}
}

func (o CategoryOrigin) Declared() bool { return o.declared }

func (o CategoryOrigin) ID() string { return o.id }

func (o CategoryOrigin) AbsentReason() string { return o.absentReason }
