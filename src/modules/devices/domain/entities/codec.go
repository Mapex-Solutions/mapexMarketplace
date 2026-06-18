package entities

// Codec is one payload codec shipped with a model: a vendor-official or community
// implementation targeting a specific platform (TTN, ChirpStack, ...). The same
// device often has several, so each is self-describing and located by Path.
type Codec struct {
	ID          string
	Name        string
	Source      string // vendor | community | platform
	Official    bool
	Default     bool
	Target      string // ttn | chirpstack | datacake | generic
	Language    string
	SourceURL   string
	Path        string // folder under the model, e.g. "codecs/milesight-official"
	DecoderFile string
	EncoderFile string
}
