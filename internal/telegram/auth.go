package telegram

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

type interactiveAuth struct {
	phone    string
	password string
	prompter *prompter
}

func (a *interactiveAuth) Phone(ctx context.Context) (string, error) {
	if a.phone != "" {
		return a.phone, nil
	}
	return a.prompter.ask("Enter Telegram phone (+country code): ")
}

func (a *interactiveAuth) Password(ctx context.Context) (string, error) {
	if a.password != "" {
		return a.password, nil
	}
	return a.prompter.askHidden("Two-factor password (leave blank if disabled): ")
}

func (a *interactiveAuth) Code(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
	if sentCode != nil && sentCode.Type != nil {
		fmt.Fprintf(a.prompter.out, "Code sent via %T\n", sentCode.Type)
	}
	return a.prompter.ask("Enter login code: ")
}

func (a *interactiveAuth) AcceptTermsOfService(ctx context.Context, tos tg.HelpTermsOfService) error {
	fmt.Fprintln(a.prompter.out, "Telegram requires accepting new Terms of Service. Please open the official Telegram app to accept them and try again.")
	return errors.New("terms of service must be accepted in the official Telegram client")
}

func (a *interactiveAuth) SignUp(ctx context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, errors.New("sign up is not supported in ttools; please register using the official Telegram client")
}

type prompter struct {
	in      io.Reader
	out     io.Writer
	err     io.Writer
	scanner *bufio.Reader
}

func newPrompter(in io.Reader, out, err io.Writer) *prompter {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}
	if err == nil {
		err = os.Stderr
	}
	return &prompter{
		in:      in,
		out:     out,
		err:     err,
		scanner: bufio.NewReader(in),
	}
}

func (p *prompter) ask(prompt string) (string, error) {
	if _, err := fmt.Fprint(p.out, prompt); err != nil {
		return "", err
	}
	input, err := p.readLine()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

func (p *prompter) askHidden(prompt string) (string, error) {
	if f, ok := p.in.(interface{ Fd() uintptr }); ok {
		if _, err := fmt.Fprint(p.out, prompt); err != nil {
			return "", err
		}
		data, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(p.out)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(data)), nil
	}
	return p.ask(prompt)
}

func (p *prompter) readLine() (string, error) {
	line, err := p.scanner.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) {
			return line, nil
		}
		return "", err
	}
	return line, nil
}
