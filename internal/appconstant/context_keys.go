package appconstant

type ctxKey string

const (
	ContextUserID    ctxKey = "userID"
	ContextProfileID ctxKey = "profileID"
	ContextContentID ctxKey = "contentID"
	ContextGroupID   ctxKey = "groupID"
	ContextToken     ctxKey = "token"
)

func (c ctxKey) String() string {
	return string(c)
}
