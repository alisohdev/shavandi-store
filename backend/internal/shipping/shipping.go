package shipping

type Method struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	FeeToman int64  `json:"fee_toman"`
	Active   bool   `json:"active"`
	Version  int64  `json:"version"`
}
