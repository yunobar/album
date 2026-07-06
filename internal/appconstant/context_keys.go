package appconstant

type ctxKey string

const (
	ContextUserID    ctxKey = "userID"
	ContextProfileID ctxKey = "profileID"
)

func (c ctxKey) String() string {
	return string(c)
}
