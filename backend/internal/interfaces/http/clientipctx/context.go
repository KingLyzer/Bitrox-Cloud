package clientipctx

import "context"

type key string

const clientIPContextKey key = "client_ip_context"

func WithClientIP(ctx context.Context, clientIP string) context.Context {
	return context.WithValue(ctx, clientIPContextKey, clientIP)
}

func FromContext(ctx context.Context) (string, bool) {
	value := ctx.Value(clientIPContextKey)
	if value == nil {
		return "", false
	}
	clientIP, ok := value.(string)
	if !ok {
		return "", false
	}
	return clientIP, true
}
