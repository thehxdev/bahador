package telbot

type Update struct {
	Id      int      `json:"update_id"`
	Message *Message `json:"message"`
}

type UpdateParams struct {
	Offset         int
	Limit          int
	Timeout        int
	AllowedUpdates []string
}

func (up *UpdateParams) ToParamsStringMap() (*ParamsStringMap, error) {
	p := &ParamsStringMap{}

	p.AddNonZeroInt("offset", up.Offset)
	p.AddNonZeroInt("limit", up.Limit)
	p.AddNonZeroInt("timeout", up.Timeout)
	err := p.AddInterface("allowed_updates", up.AllowedUpdates)
	if err != nil {
		return nil, err
	}

	return p, nil
}
