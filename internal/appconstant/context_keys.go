package appconstant

type ctxKey string

const (
	ContextUserID    ctxKey = "userID"
	ContextProfileID ctxKey = "profileID"
	ContextContentID ctxKey = "contentID"
)

func (c ctxKey) String() string {
	return string(c)
}
