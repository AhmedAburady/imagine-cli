package openai

// Interactive "Sign in with ChatGPT" browser flow, wired as the subscription
// AuthMethod's Login (see register.go): spin a loopback listener, open the
// browser to the authorize URL, capture the redirect, exchange the code, and
// persist the tokens.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

// subscriptionLogin runs the OAuth/PKCE sign-in and writes the tokens to the
// store. Honors ctx cancellation (Ctrl+C aborts a pending sign-in).
func subscriptionLogin(ctx context.Context, out io.Writer) error {
	pk, err := newPKCE()
	if err != nil {
		return err
	}
	state, err := randomState()
	if err != nil {
		return err
	}

	lns, port, err := listenCallback()
	if err != nil {
		return err
	}
	defer func() {
		for _, ln := range lns {
			_ = ln.Close()
		}
	}()

	redirectURI := fmt.Sprintf("http://localhost:%d%s", port, callbackPath)
	authURL := buildAuthorizeURL(pk.challenge, state, redirectURI)

	resultCh := make(chan callbackResult, 1)
	srv := &http.Server{Handler: callbackMux(state, resultCh)}
	for _, ln := range lns {
		go func(l net.Listener) { _ = srv.Serve(l) }(ln)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	fmt.Fprintf(out, "\n  Opening your browser to sign in with ChatGPT…\n  If it doesn't open automatically, visit:\n\n    %s\n\n", authURL)
	if oerr := openBrowser(authURL); oerr != nil {
		fmt.Fprintf(out, "  (couldn't launch a browser automatically: %v — paste the URL above)\n\n", oerr)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case res := <-resultCh:
		if res.err != nil {
			return res.err
		}
		tr, xerr := exchangeCode(ctx, res.code, pk.verifier, redirectURI)
		if xerr != nil {
			return fmt.Errorf("token exchange failed: %w", xerr)
		}

		var sa storedAuth
		sa.Tokens.AccessToken = tr.AccessToken
		sa.Tokens.IDToken = tr.IDToken
		sa.Tokens.RefreshToken = tr.RefreshToken
		sa.Tokens.AccountID = accountIDFromIDToken(tr.IDToken)
		sa.LastRefresh = time.Now()

		path := loginStorePath()
		if serr := saveAuth(path, &sa); serr != nil {
			return serr
		}

		// Let the browser fetch /success before the deferred shutdown.
		time.Sleep(600 * time.Millisecond)
		fmt.Fprintf(out, "  Signed in. Credentials stored at %s\n", path)
		return nil
	}
}

type callbackResult struct {
	code string
	err  error
}

// callbackMux validates state and captures the code at /auth/callback, then
// bounces the browser to /success.
func callbackMux(state string, resultCh chan<- callbackResult) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		// State first: a stray local request (e.g. ?error=denied with no valid
		// state) must not be able to abort an in-flight sign-in.
		if q.Get("state") != state {
			browserMessage(w, "Sign-in failed (state mismatch). You can close this window.")
			resultCh <- callbackResult{err: errors.New("OAuth state mismatch — sign-in aborted for safety")}
			return
		}
		if e := q.Get("error"); e != "" {
			browserMessage(w, "Sign-in failed. You can close this window and return to the terminal.")
			resultCh <- callbackResult{err: fmt.Errorf("authorization denied: %s %s", e, q.Get("error_description"))}
			return
		}
		code := q.Get("code")
		if code == "" {
			browserMessage(w, "Sign-in failed (no code returned). You can close this window.")
			resultCh <- callbackResult{err: errors.New("no authorization code in callback")}
			return
		}
		http.Redirect(w, r, successPath, http.StatusFound)
		resultCh <- callbackResult{code: code}
	})
	mux.HandleFunc(successPath, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, successHTML)
	})
	return mux
}

// listenCallback binds the primary port, then the fallback — Codex keeps both
// on its redirect-URI allow-list, so an arbitrary free port won't work. It
// binds both loopback families (127.0.0.1 and ::1) so the browser's redirect to
// http://localhost reaches us regardless of how localhost resolves on the host.
func listenCallback() ([]net.Listener, int, error) {
	for _, port := range []int{callbackPort, callbackPortFallback} {
		// 127.0.0.1 is what localhost resolves to on virtually every host, so
		// we must own it on the chosen port — if it's taken, move to the next
		// port rather than half-owning this one and handing the browser a port
		// where someone else answers on IPv4.
		v4, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue
		}
		lns := []net.Listener{v4}
		// ::1 is best-effort, for hosts that resolve localhost to IPv6.
		if v6, verr := net.Listen("tcp", fmt.Sprintf("[::1]:%d", port)); verr == nil {
			lns = append(lns, v6)
		}
		return lns, port, nil
	}
	return nil, 0, fmt.Errorf("could not bind localhost port %d or %d for the sign-in callback — close whatever is using them and retry", callbackPort, callbackPortFallback)
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

func browserMessage(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	_, _ = fmt.Fprintf(w, "<!doctype html><meta charset=utf-8><body style=\"font-family:system-ui;padding:3rem;text-align:center\"><p>%s</p></body>", msg)
}

const successHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>imagine — signed in</title>
<style>
  :root { color-scheme: dark; }
  html, body { height: 100%; margin: 0; }
  body {
    background: #0a0a0a;
    color: #fff;
    font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
    display: flex; align-items: center; justify-content: center;
    padding: 2rem;
    -webkit-font-smoothing: antialiased;
  }
  .card { display: flex; flex-direction: column; align-items: center; text-align: center; gap: 1.8rem; animation: rise .5s ease-out both; }
  .logo {
    width: 104px; height: auto;
    filter: drop-shadow(0 10px 40px rgba(0,0,0,.55)) hue-rotate(0deg);
    animation: hue 5s linear infinite;
  }
  .text { display: flex; flex-direction: column; align-items: center; gap: .55rem; }
  h1 { font-size: 1.6rem; font-weight: 600; margin: 0; color: #fff; letter-spacing: -.01em; }
  p { margin: 0; max-width: 24rem; color: #fff; font-weight: 400; font-size: .95rem; line-height: 1.55; }
  @keyframes rise { from { opacity: 0; transform: translateY(10px); } to { opacity: 1; transform: none; } }
  @keyframes hue {
    from { filter: drop-shadow(0 10px 40px rgba(0,0,0,.55)) hue-rotate(0deg); }
    to   { filter: drop-shadow(0 10px 40px rgba(0,0,0,.55)) hue-rotate(360deg); }
  }
  @media (prefers-reduced-motion: reduce) { .card, .logo { animation: none; } }
</style>
</head>
<body>
  <main class="card">
    <img class="logo" src="https://pub-eeb4782b194e4eaf9c6af8206409f66e.r2.dev/imagine.png" alt="imagine" />
    <div class="text">
      <h1>Your ChatGPT subscription is connected.</h1>
      <p>You can close this window and return to the terminal.</p>
    </div>
  </main>
</body>
</html>`
