package mwclient

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// Edit takes a map[string]string containing parameters for an edit action and
// attempts to perform the edit. Edit will return nil if no errors are detected.
// The editcfg map[string]string argument should contain parameters from:
//	https://www.mediawiki.org/wiki/API:Edit#Parameters
// Edit will set the 'action' and 'token' parameters automatically, but if the token
// field in editcfg is non-empty, Edit will not override it.
// Edit does not check editcfg for sanity.
// editcfg example:
//	map[string]string{
//		"pageid":   "709377",
//		"text":     "Complete new text for page",
//		"summary":  "Take that, page!",
//		"notminor": "",
//	}
func (w *Client) Edit(editcfg map[string]string) error {
	// If edit token not set, obtain one from API or cache
	if editcfg["token"] == "" {
		editToken, err := w.GetToken("edit")
		if err != nil {
			return fmt.Errorf("unable to obtain edit token: %s", err)
		}
		editcfg["token"] = editToken
	}

	params := url.Values{}
	for k, v := range editcfg {
		params.Set(k, v)
	}
	params.Set("action", "edit")

	resp, err := w.Post(params)
	if err != nil {
		return err
	}

	editResult, err := resp.GetPath("edit", "result").String()
	if err != nil {
		return fmt.Errorf("unable to assert 'result' field to type string\n")
	}

	if editResult != "Success" {
		if captcha, ok := resp.Get("edit").CheckGet("captcha"); ok {
			captchaBytes, err := captcha.Encode()
			if err != nil {
				return fmt.Errorf("error occured while creating error message: %s", err)
			}
			var captchaerr captchaError
			err = json.Unmarshal(captchaBytes, &captchaerr)
			if err != nil {
				return fmt.Errorf("error occured while creating error message: %s", err)
			}
			return captchaerr
		}

		return fmt.Errorf("unrecognized response: %v", resp.Get("edit"))
	}

	return nil
}

// getPage gets the content of a page and the timestamp of its most recent revision.
// The page is specified either by its name or by its ID.
// If the isName parameter is true, then the pageIDorName parameter will be
// assumed to be a page name and vice versa.
func (w *Client) getPage(pageIDorName string, isName bool) (content string, timestamp string, err error) {
	parameters := url.Values{
		"action":       {"query"},
		"prop":         {"revisions"},
		"rvprop":       {"content|timestamp"},
		"indexpageids": {""},
	}

	if isName {
		parameters.Set("titles", pageIDorName)
	} else {
		parameters.Set("pageids", pageIDorName)
	}

	resp, err := w.Get(parameters)
	if err != nil {
		return "", "", err
	}

	pageIDs, err := resp.GetPath("query", "pageids").Array()
	if err != nil {
		return "", "", err
	}
	id := pageIDs[0].(string)

	rv := resp.GetPath("query", "pages", id).Get("revisions").GetIndex(0)

	content, err = rv.Get("*").String()
	if err != nil {
		// I don't know when this would ever happen, but just to be safe...
		return "", "", fmt.Errorf("unable to assert page content to string: %s", err)
	}

	timestamp, err = rv.Get("timestamp").String()
	if err != nil {
		return "", "", fmt.Errorf("unable to assert timestamp to string: %s", err)
	}

	return content, timestamp, nil
}

// GetPageByName gets the content of a page (specified by its name) and
// the timestamp of its most recent revision.
func (w *Client) GetPageByName(pageName string) (content string, timestamp string, err error) {
	return w.getPage(pageName, true)
}

// GetPageByID gets the content of a page (specified by its id) and
// the timestamp of its most recent revision.
func (w *Client) GetPageByID(pageID string) (content string, timestamp string, err error) {
	return w.getPage(pageID, false)
}

// GetToken returns a specified token (and an error if this is not possible).
// If the token is not already available in the Client.Tokens map,
// it will attempt to retrieve it via the API.
// tokenName should be "edit" (or whatever), not "edittoken".
func (w *Client) GetToken(tokenName string) (string, error) {
	if _, ok := w.Tokens[tokenName]; ok {
		return w.Tokens[tokenName], nil
	}

	parameters := url.Values{
		"action": {"tokens"},
		"type":   {tokenName},
	}

	resp, err := w.Get(parameters)
	if err != nil {
		return "", err
	}

	token, err := resp.GetPath("tokens", tokenName+"token").String()
	if err != nil {
		// This really shouldn't happen.
		return "", fmt.Errorf("error occured while converting token to string: %s", err)
	}
	w.Tokens[tokenName] = token
	return token, nil
}
