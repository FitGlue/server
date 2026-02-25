package email

import (
	"fmt"
	"strings"
	"time"
)

type LayoutOptions struct {
	Content     string
	PreviewText string
	BaseURL     string
}

// RenderLayout wraps email content in the branded FitGlue layout.
func RenderLayout(opts LayoutOptions) string {
	year := time.Now().Year()

	tmpl := `<!DOCTYPE html>
<html lang="en" xmlns="http://www.w3.org/1999/xhtml">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta http-equiv="X-UA-Compatible" content="IE=edge">
  <title>FitGlue</title>
  <!--[if mso]>
  <noscript><xml><o:OfficeDocumentSettings><o:PixelsPerInch>96</o:PixelsPerInch></o:OfficeDocumentSettings></xml></noscript>
  <![endif]-->
</head>
<body style="margin:0;padding:0;background-color:%s;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Oxygen,Ubuntu,Cantarell,'Helvetica Neue',Arial,sans-serif;-webkit-font-smoothing:antialiased;">
  <!-- Preview text (hidden) -->
  <div style="display:none;max-height:0;overflow:hidden;mso-hide:all;">%s</div>
  <div style="display:none;max-height:0;overflow:hidden;mso-hide:all;">&zwnj;&nbsp;&zwnj;&nbsp;&zwnj;&nbsp;&zwnj;&nbsp;&zwnj;&nbsp;&zwnj;&nbsp;&zwnj;&nbsp;&zwnj;&nbsp;</div>

  <!-- Outer wrapper -->
  <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="background-color:%s;">
    <tr>
      <td align="center" style="padding:40px 16px;">
        <!-- Inner card -->
        <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="600" style="max-width:600px;width:100%%;">

          <!-- HEADER -->
          <tr>
            <td style="background:%s;padding:32px 40px;border-radius:16px 16px 0 0;text-align:center;">
              <a href="%s" style="text-decoration:none;">
                <span style="font-size:32px;font-weight:900;letter-spacing:-0.02em;">
                  <span style="color:%s;">Fit</span><span style="color:%s;">Glue</span>
                </span>
              </a>
            </td>
          </tr>

          <!-- BODY -->
          <tr>
            <td style="background:%s;padding:40px;border-left:1px solid %s;border-right:1px solid %s;">
              %s
            </td>
          </tr>

          <!-- FOOTER -->
          <tr>
            <td style="background:%s;padding:24px 40px;border-radius:0 0 16px 16px;border:1px solid %s;border-top:none;">
              <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%">
                <tr>
                  <td style="text-align:center;padding-bottom:12px;">
                    <a href="%s" style="color:%s;font-size:12px;text-decoration:none;margin:0 8px;">Website</a>
                    <span style="color:%s;">•</span>
                    <a href="https://discord.gg/fitglue" style="color:%s;font-size:12px;text-decoration:none;margin:0 8px;">Community</a>
                    <span style="color:%s;">•</span>
                    <a href="mailto:support@fitglue.tech" style="color:%s;font-size:12px;text-decoration:none;margin:0 8px;">Support</a>
                  </td>
                </tr>
                <tr>
                  <td style="text-align:center;">
                    <p style="color:%s;font-size:11px;margin:0;line-height:1.5;">
                      © %d FitGlue. Your fitness data, your way.
                    </p>
                  </td>
                </tr>
              </table>
            </td>
          </tr>

        </table>
      </td>
    </tr>
  </table>
</body>
</html>`

	return fmt.Sprintf(tmpl,
		Brand.BgBody,
		opts.PreviewText,
		Brand.BgBody,
		Brand.BgDark,
		opts.BaseURL,
		Brand.Primary,
		Brand.Secondary,
		Brand.BgCard,
		Brand.Border,
		Brand.Border,
		opts.Content,
		Brand.FooterBg,
		Brand.Border,
		opts.BaseURL,
		Brand.TextMuted,
		Brand.Border,
		Brand.TextMuted,
		Brand.Border,
		Brand.TextMuted,
		Brand.TextMuted,
		year,
	)
}

func ctaButton(text, url string) string {
	tmpl := `<table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="margin:32px 0;">
  <tr>
    <td align="center">
      <a href="%s" target="_blank" style="display:inline-block;padding:16px 40px;background:linear-gradient(135deg,%s,%s);color:#ffffff;text-decoration:none;border-radius:10px;font-weight:700;font-size:16px;letter-spacing:0.02em;mso-padding-alt:16px 40px;">
        %s
      </a>
    </td>
  </tr>
</table>`
	return fmt.Sprintf(tmpl, url, Brand.Primary, Brand.Secondary, text)
}

func heading(text string) string {
	return fmt.Sprintf(`<h1 style="color:%s;font-size:24px;font-weight:700;margin:0 0 16px;line-height:1.3;">%s</h1>`, Brand.TextPrimary, text)
}

func paragraph(text string) string {
	return fmt.Sprintf(`<p style="color:%s;font-size:16px;line-height:1.6;margin:0 0 16px;">%s</p>`, Brand.TextSecondary, text)
}

func smallText(text string) string {
	return fmt.Sprintf(`<p style="color:%s;font-size:13px;line-height:1.5;margin:16px 0 0;">%s</p>`, Brand.TextMuted, text)
}

func divider() string {
	return fmt.Sprintf(`<hr style="border:none;border-top:1px solid %s;margin:24px 0;">`, Brand.Border)
}

func emoji(char string) string {
	return fmt.Sprintf(`<span style="font-size:48px;display:block;text-align:center;margin-bottom:16px;">%s</span>`, char)
}

func joinContent(parts ...string) string {
	return strings.Join(parts, "\n")
}
