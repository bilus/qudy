package generator

// generateDispatchEntry writes the table's entry for one handler, on the
// receiver or on the nested instance the steps lead to.
func (g *liveGenerator) generateDispatchEntry(name string, steps []navStep, recv string, h handlerBinding) {
	params := valueParams(h.sig)
	event := "_"
	if len(params) > 0 || h.fill != "" {
		event = GENSYM("e")
	}
	var locals []string
	//`~{strconv.Quote(name)}: func(lv live.Ctx, ~event live.Event) error {
	for _, p := range params {
		local := GENSYM(p.Name)
		locals = append(locals, local)
		//`~local, errName# := live.Param[~p.Type.Text](~event, ~{strconv.Quote(p.Name)})
		//`if errName# != nil {
		//`	return errName#
		//`}
	}
	for i, s := range steps {
		if i > 0 {
			//`{
		}
		//`~s.param := ~s.state
		recv = s.param
	}
	fill, callee := h.fill, h.method
	if h.field != "" {
		callee = h.field + "." + h.method
	}
	if fill != "" {
		//`if decodeErr# := form.Decode(&~recv.~fill, ~event); decodeErr# != nil {
		//`	return decodeErr#
		//`}
		//`if ruleErr# := form.Validate(&~recv.~fill); ruleErr# != nil {
		//`	return ruleErr#
		//`}
		//`return form.Update(&~recv.~fill, func() {
	}
	//`~recv.~callee(lv\
	for _, s := range steps {
		if s.id != "" {
			//`.WithID(~{strconv.Quote(s.id)})\
		}
	}
	for _, local := range locals {
		//`, ~local\
	}
	//`)
	if fill != "" {
		//`}) ~// close the Update
	} else {
		//`return nil
	}
	for range max(len(steps)-1, 0) {
		//`} ~// close the steps
	}
	//`}, ~// close the map entry
}
