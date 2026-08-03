package pi

type sessionReceipt struct {
	Contract       string          `json:"contract"`
	RuntimeLocator string          `json:"runtime_locator"`
	Identity       AdapterIdentity `json:"identity"`
}
