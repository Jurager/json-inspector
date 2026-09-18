package accountclient

import (
	"context"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	accountv1 "github.com/Jurager/json-inspector-proto/go/account/v1"

	"json-inspector/internal/domain"
)

// The account API, in the app's own words. Every method here is a thin reading of one call — the
// decisions about when to make it are the use case's, and this is only the translation.

// Me is who the account is, as the server describes it. The session id in the answer is what makes
// this device recognisable in the list of them, and the plan is what tells the app whether there is
// a tariff worth drawing at all.
func (c *Client) Me(ctx context.Context, address, access string) (domain.Account, error) {
	answer, err := call(c, ctx, address, access,
		func(ctx context.Context, client accountv1.AccountServiceClient) (*accountv1.MeResponse, error) {
			return client.Me(ctx, &accountv1.MeRequest{})
		})
	if err != nil {
		return domain.Account{}, err
	}

	account := answer.GetAccount()
	return domain.Account{
		UserID:    account.GetId(),
		Email:     account.GetEmail(),
		Name:      account.GetName(),
		Plan:      account.GetPlan(),
		Seats:     int(account.GetSeats()),
		SessionID: answer.GetSessionId(),
	}, nil
}

func (c *Client) Sessions(ctx context.Context, address, access string) ([]domain.Session, error) {
	answer, err := call(c, ctx, address, access,
		func(ctx context.Context,
			client accountv1.AccountServiceClient) (*accountv1.ListSessionsResponse, error) {
			return client.ListSessions(ctx, &accountv1.ListSessionsRequest{})
		})
	if err != nil {
		return nil, err
	}
	return sessions(answer.GetSessions()), nil
}

// EndSession ends one device. The answer is the list that is left, so that the window redraws
// without asking again.
func (c *Client) EndSession(ctx context.Context, address, access, sessionID string) (
	[]domain.Session, error) {
	answer, err := call(c, ctx, address, access,
		func(ctx context.Context,
			client accountv1.AccountServiceClient) (*accountv1.EndSessionResponse, error) {
			return client.EndSession(ctx,
				accountv1.EndSessionRequest_builder{SessionId: sessionID}.Build())
		})
	if err != nil {
		return nil, err
	}
	return sessions(answer.GetSessions()), nil
}

func (c *Client) EndOthers(ctx context.Context, address, access string) ([]domain.Session, error) {
	answer, err := call(c, ctx, address, access,
		func(ctx context.Context,
			client accountv1.AccountServiceClient) (*accountv1.EndOtherSessionsResponse, error) {
			return client.EndOtherSessions(ctx, &accountv1.EndOtherSessionsRequest{})
		})
	if err != nil {
		return nil, err
	}
	return sessions(answer.GetSessions()), nil
}

// SignOut ends this device's session on the server. It is called on the way out and its answer is
// nothing: the app has already decided to forget the account, and a server that disagrees is a
// server to sort out the next time somebody signs in.
func (c *Client) SignOut(ctx context.Context, address, access string) error {
	_, err := call(c, ctx, address, access,
		func(ctx context.Context,
			client accountv1.AccountServiceClient) (*accountv1.SignOutResponse, error) {
			return client.SignOut(ctx, &accountv1.SignOutRequest{})
		})
	return err
}

// DeleteAccount leaves the server for good: the account and every device on it.
func (c *Client) DeleteAccount(ctx context.Context, address, access string) error {
	_, err := call(c, ctx, address, access,
		func(ctx context.Context,
			client accountv1.AccountServiceClient) (*accountv1.DeleteAccountResponse, error) {
			return client.DeleteAccount(ctx, &accountv1.DeleteAccountRequest{})
		})
	return err
}

// sessions is the contract's list as the app's own.
func sessions(rows []*accountv1.Session) []domain.Session {
	out := make([]domain.Session, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Session{
			ID: row.GetId(),
			Device: domain.Device{
				Name:       row.GetDevice().GetName(),
				Platform:   row.GetDevice().GetPlatform(),
				AppVersion: row.GetDevice().GetAppVersion(),
			},
			IP:         row.GetIp(),
			CreatedAt:  moment(row.GetCreatedAt()),
			LastSeenAt: moment(row.GetLastSeenAt()),
		})
	}
	return out
}

// moment is a timestamp that may not be there at all: the server leaves one out rather than sending
// 1970, and a session without one is a session whose time nobody knows.
func moment(stamp *timestamppb.Timestamp) time.Time {
	if stamp == nil {
		return time.Time{}
	}
	return stamp.AsTime()
}

// refuse turns what a call failed with into the app's own refusal.
//
// What matters to everything above is one distinction: a server that answered has an opinion — and
// the opinion decides whether the account is still good — while a server that did not answer is
// merely away, and the next click may well work.
func refuse(err error) error {
	if err == nil {
		return nil
	}
	if domain.CodeOf(err) != "" {
		return err
	}
	return domain.Refuse(domain.CodeServerUnreachable, nil, nil)
}

// fromStatus reads a gRPC failure. The codes the server sends carry their reason in the status
// details — that is the whole point of the shape — and the two the app acts on are "you are not
// signed in any more" and "the server is not there".
func fromStatus(err error) error {
	if err == nil {
		return nil
	}
	if domain.CodeOf(err) != "" {
		return err
	}

	held := status.Convert(err)
	switch held.Code() {
	case codes.OK:
		return nil
	case codes.Unavailable, codes.DeadlineExceeded, codes.Canceled:
		return domain.Refuse(domain.CodeServerUnreachable, nil, nil)
	}

	switch reason(held) {
	case "unauthorized", "sessionGone":
		// The session this device was signed in on is gone: ended from another one, or expired. The
		// app's move is to forget it, and that is what this code tells the use case to do.
		return domain.Refuse(domain.CodeNotSignedIn, domain.ErrNotAllowed, nil)
	case "notFound":
		return domain.Refuse(domain.CodeNotFound, domain.ErrNotFound, nil)
	}
	return domain.Refuse(domain.CodeServerRefused, nil, nil)
}

// reason is the app's code the server put in the status, if it put one there.
func reason(held *status.Status) string {
	for _, detail := range held.Details() {
		if info, ok := detail.(*errdetails.ErrorInfo); ok {
			return info.GetReason()
		}
	}
	return ""
}
