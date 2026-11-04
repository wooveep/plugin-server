---
title: OIDC Authentication
keywords: [higress, oidc]
description: OIDC Authentication Plugin Configuration Reference
---
## Function Description

All `YOUR_*` values below are placeholders. Replace them with your environment settings before use. Generate `YOUR_BASE64_COOKIE_SECRET` with `openssl rand -base64 32`, and keep client secrets, tokens and cookies out of source control.
This plugin supports OpenID Connect (OIDC) identity authentication. Additionally, it enhances defenses against Cross-Site Request Forgery (CSRF) attacks and supports the Logout Endpoint and Refresh Token mechanism in the OpenID Connect protocol. Requests verified by the Wasm plugin after OIDC validation will carry the `Authorization` header, including the corresponding Access Token.

## Running Attributes
Plugin execution phase: `authentication phase`

Plugin execution priority: `350`

## Configuration Fields
| Option                        | Type         | Description                                                  | Default           |
| ----------------------------- | ------------ | ------------------------------------------------------------ | ----------------- |
| cookie_name                   | string       | The name of the cookie that the oauth_proxy creates. Should be changed to use a [cookie prefix](https://developer.mozilla.org/en-US/docs/Web/HTTP/Cookies#cookie_prefixes) (`__Host-` or `__Secure-`) if `--cookie-secure` is set. | `"_oauth2_proxy"` |
| cookie_secret                 | string       | The seed string for secure cookies (optionally base64 encoded) |                   |
| cookie_domains                | string\|list | Optional cookie domains to force cookies to (e.g. `.yourcompany.com`). The longest domain matching the request's host will be used (or the shortest cookie domain if there is no match). |                   |
| cookie_path                   | string       | An optional cookie path to force cookies to (e.g. `/poc/`)   | `"/"`             |
| cookie_expire                 | duration     | Expire timeframe for cookie. If set to 0, cookie becomes a session-cookie which will expire when the browser is closed. | 168h0m0s          |
| cookie_refresh                | duration     | Refresh the cookie after this duration; `0` to disable       |                   |
| cookie_secure                 | bool         | Set [secure (HTTPS only) cookie flag](https://owasp.org/www-community/controls/SecureFlag) | true              |
| cookie_httponly               | bool         | Set HttpOnly cookie flag                                     | true              |
| cookie_samesite               | string       | Set SameSite cookie attribute (`"lax"`, `"strict"`, `"none"`, or `""`). | `""`              |
| cookie_csrf_per_request       | bool         | Enable having different CSRF cookies per request, making it possible to have parallel requests. | false             |
| cookie_csrf_expire            | duration     | Expire timeframe for CSRF cookie                             | 15m               |
| client_id                     | string       | The OAuth Client ID                                          |                   |
| client_secret                 | string       | The OAuth Client Secret                                      |                   |
| provider                      | string       | OAuth provider                                               | oidc              |
| pass_authorization_header     | bool         | Pass OIDC IDToken to upstream via Authorization Bearer header | true              |
| oidc_issuer_url               | string       | The OpenID Connect issuer URL, e.g. `"https://YOUR_AUTH0_DOMAIN"` |                   |
| oidc_verifier_request_timeout | uint32       | OIDC verifier discovery request timeout                      | 2000(ms)          |
| scope                         | string       | OAuth scope specification                                    |                   |
| redirect_url                  | string       | The OAuth Redirect URL, e.g. `"https://internalapp.yourcompany.com/oauth2/callback"` |                   |
| service_name                  | string       | Registered name of the OIDC service, e.g. `auth.dns`, `keycloak.static` |                   |
| service_port                  | int64        | Service port of the OIDC service                             |                   |
| service_host                  | string       | Host of the OIDC service when type is static IP              |                   |
| match_type                    | string       | Match type (`whitelist` or `blacklist`)                      | `"whitelist"`     |
| match_list                    | rule\|list   | A list of (match_rule_domain, match_rule_path, and match_rule_type). |                   |
| match_rule_domain             | string       | Match rule domain, support wildcard pattern such as `*.bar.com` |                   |
| match_rule_path               | string       | Match rule path such as `/headers`                           |                   |
| match_rule_type               | string       | Match rule type can be `exact` or `prefix` or `regex`        |                   |

## Usage
### Generate Cookie Secret
``` python
python -c 'import os, base64; print(base64.urlsafe_b64encode(os.urandom(32)).decode())'
```

Reference: [Oauth2-proxy Generating a Cookie Secret](https://oauth2-proxy.github.io/oauth2-proxy/configuration/overview#generating-a-cookie-secret)

### Whitelist and Blacklist Mode
Supports whitelist and blacklist configuration, defaulting to whitelist mode, which is empty, meaning all requests need to be validated. Domain matching supports wildcard domains like `*.bar.com`, and matching rules support exact match `exact`, prefix match `prefix`, and regex match `regex`.

* **Whitelist Mode**
```yaml
match_type: 'whitelist'
match_list:
    - match_rule_domain: '*.bar.com'
      match_rule_path: '/foo'
      match_rule_type: 'prefix'
```

Requests matching the prefix `/foo` under the wildcard domain `*.bar.com` do not need validation.

* **Blacklist Mode**
```yaml
match_type: 'blacklist'
match_list:
    - match_rule_domain: '*.bar.com'
      match_rule_path: '/headers'
      match_rule_type: 'prefix'
```

Only requests matching the prefix `/headers` under the wildcard domain `*.bar.com` need validation.

### Logout Users
To log out users, they must be redirected to the `/oauth2/sign_out` endpoint. This endpoint only removes cookies set by oauth2-proxy, meaning the user remains logged into the OIDC Provider and may be automatically logged back in upon re-accessing the application. Therefore, the `rd` query parameter must be used to redirect the user to the authentication provider's logout page, redirecting the user to a URL similar to the following (note URL encoding!):

```
/oauth2/sign_out?rd=https%3A%2F%2Fmy-oidc-provider.example.com%2Fsign_out_page
```

Alternatively, the redirect URL can be included in the `X-Auth-Request-Redirect` header:

```
GET /oauth2/sign_out HTTP/1.1
X-Auth-Request-Redirect: https://my-oidc-provider.example.com/sign_out_page
...
```

The redirect URL can include the `post_logout_redirect_uri` parameter to specify the page to redirect to after OIDC Provider logout; for example, the backend service's logout page. If this parameter is not included, it will default to redirect to the OIDC Provider's logout page. For details, see the examples below for Auth0 and Keycloak (if the OIDC Provider supports session management and discovery, then "sign_out_page" should be obtained from the [metadata](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfig) `end_session_endpoint`).

### OIDC Service HTTPS Protocol
If the OIDC Provider uses the HTTPS protocol, refer to Higress's documentation on [Configuring Backend Service Protocol: HTTPS](https://higress.io/docs/latest/user/annotation-use-case/#%E9%85%8D%E7%BD%AE%E5%90%8E%E7%AB%AF%E6%9C%8D%E5%8A%A1%E5%8D%8F%E8%AE%AEhttps%E6%88%96grpc), which states that requests forwarded to the backend service should use the HTTPS protocol by using the annotation `higress.io/backend-protocol: "HTTPS"`, as shown in the Auth0 example Ingress configuration.

## Configuration Example
### Auth0 Configuration Example
#### Step 1: Configure Auth0 Account
- Log in to the Developer Okta website [Developer Auth0 site](https://auth0.com/)
- Register a test web application
**Note**: You must fill in the Allowed Callback URLs, Allowed Logout URLs, Allowed Web Origins, etc., otherwise the OIDC Provider will consider the user's redirect URL or logout URL to be invalid.

#### Step 2: Higress Configure Service Source
* Create an Auth0 DNS source in Higress service sources.
![auth0 create](https://gw.alicdn.com/imgextra/i1/O1CN01p9y0jF1tfzdXTzNYm_!!6000000005930-0-tps-3362-670.jpg)

#### Step 3: OIDC Service HTTPS Configuration
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: auth0-ingress
  annotations:
    higress.io/destination: auth.dns
    higress.io/backend-protocol: "HTTPS"
    higress.io/ignore-path-case: "false"
spec:
  ingressClassName: higress
  rules:
    - host: foo.bar.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              resource:
                apiGroup: networking.higress.io
                kind: McpBridge
                name: default
```

#### Step 4: Wasm Plugin Configuration
```yaml
redirect_url: 'http://foo.bar.com/oauth2/callback'
oidc_issuer_url: 'https://YOUR_AUTH0_DOMAIN/'
client_id: 'YOUR_CLIENT_ID'
client_secret: 'YOUR_CLIENT_SECRET'
scope: 'openid email offline_access'
cookie_secret: 'YOUR_BASE64_COOKIE_SECRET'
service_name: 'auth.dns'
service_port: 443
match_type: 'whitelist'
match_list:
    - match_rule_domain: '*.bar.com'
      match_rule_path: '/foo'
      match_rule_type: 'prefix'
```

**Note**: You must configure the service source first. The Wasm plugin needs to access the configured service to obtain OpenID configuration during initialization.

#### Access Service Page; Redirect if Not Logged In
![auth0_login](https://gw.alicdn.com/imgextra/i3/O1CN01hVNk0C1gkUWLwuC0N_!!6000000004180-0-tps-3840-2160.jpg)

#### Successful Login Redirects to Service Page
In the headers, you can see the `_oauth2_proxy` cookie carried for the next login access, and the Authorization corresponds to the IDToken used to get user information from the backend service.
![auth0 service](https://gw.alicdn.com/imgextra/i1/O1CN01vyrB6u1xPHep1RRqb_!!6000000006435-2-tps-3840-2160.png)

#### Access Logout Redirects to Logout Page
```
http://foo.bar.com/oauth2/sign_out?rd=https%3A%2F%2FYOUR_AUTH0_DOMAIN%2Foidc%2Flogout
```

![auth0 logout](https://gw.alicdn.com/imgextra/i3/O1CN01UntF4x1UqC4StMqtT_!!6000000002568-0-tps-3840-2160.jpg)

#### Access Logout Redirects to Logout Page (with post_logout_redirect_uri Parameter to Redirect to Specified URI)
```
http://foo.bar.com/oauth2/sign_out?rd=https%3A%2F%2FYOUR_AUTH0_DOMAIN%2Foidc%2Flogout%3Fpost_logout_redirect_uri%3Dhttp%3A%2F%2Ffoo.bar.com%2Ffoo
```

Note: The URI to which `post_logout_redirect_uri` redirects needs to be configured in the OIDC Provider Allowed URLs for a normal redirection.

![auth0 logout redirect](https://gw.alicdn.com/imgextra/i1/O1CN01AtZ2cd1JlBxsgyCjG_!!6000000001068-0-tps-3840-2160.jpg)

### Keycloak Configuration Example
#### Step 1: Get Started with Keycloak on Docker
<https://www.keycloak.org/getting-started/getting-started-docker>
**Note**: You must fill in Valid redirect URIs, Valid post logout URIs, Web origins, otherwise the OIDC Provider will consider the user's redirect URL or logout URL to be invalid.

#### Step 2: Higress Configure Service Source
* Create a Keycloak fixed address service in Higress service sources.
![keycloak create](https://gw.alicdn.com/imgextra/i1/O1CN01p9y0jF1tfzdXTzNYm_!!6000000005930-0-tps-3362-670.jpg)

#### Step 3: Wasm Plugin Configuration
```yaml
redirect_url: 'http://foo.bar.com/oauth2/callback'
oidc_issuer_url: 'http://127.0.0.1:9090/realms/myrealm'
client_id: 'YOUR_CLIENT_ID'
client_secret: 'YOUR_CLIENT_SECRET'
scope: 'openid email'
cookie_secret: 'YOUR_BASE64_COOKIE_SECRET'
service_name: 'keycloak.static'
service_port: 80
service_host: '127.0.0.1:9090'
match_type: 'blacklist'
match_list:
    - match_rule_domain: '*.bar.com'
      match_rule_path: '/headers'
      match_rule_type: 'prefix'
```

#### Access Service Page; Redirect if Not Logged In
![keycloak_login](https://gw.alicdn.com/imgextra/i4/O1CN01HLcl7r1boXwwnzGqA_!!6000000003512-0-tps-3840-2160.jpg)

#### Successful Login Redirects to Service Page
![keycloak service](https://gw.alicdn.com/imgextra/i1/O1CN01vyrB6u1xPHep1RRqb_!!6000000006435-2-tps-3840-2160.png)

#### Access Logout Redirects to Logout Page
```
http://foo.bar.com/oauth2/sign_out?rd=http%3A%2F%2F127.0.0.1:9090%2Frealms%2Fmyrealm%2Fprotocol%2Fopenid-connect%2Flogout
```

![keycloak logout](https://gw.alicdn.com/imgextra/i4/O1CN01kQwqB523OiroOWMgM_!!6000000007246-0-tps-3840-2160.jpg)

#### Access Logout Redirects to Logout Page (with post_logout_redirect_uri Parameter to Redirect to Specified URI)
```
http://foo.bar.com/oauth2/sign_out?rd=http%3A%2F%2F127.0.0.1:9090%2Frealms%2Fmyrealm%2Fprotocol%2Fopenid-connect%2Flogout%3Fpost_logout_redirect_uri%3Dhttp%3A%2F%2Ffoo.bar.com%2Ffoo
```

![keycloak logout redirect](https://gw.alicdn.com/imgextra/i1/O1CN01AtZ2cd1JlBxsgyCjG_!!6000000001068-0-tps-3840-2160.jpg)

### Aliyun Configuration Example
#### Step 1: Configure Aliyun OAuth Application
Refer to the [Web Application Login to Alibaba Cloud](https://help.aliyun.com/zh/ram/user-guide/access-alibaba-cloud-apis-from-a-web-application) process to configure the OAuth application.

#### Step 2: Higress Configure Service Source
* Create an Aliyun DNS service in Higress service sources.
![Aliyun service](https://gw.alicdn.com/imgextra/i3/O1CN01PMNGFS1mHXBtsEvEq_!!6000000004929-0-tps-3312-718.jpg)

#### Step 3: Wasm Plugin Configuration
```yaml
redirect_url: 'http://foo.bar.com/oauth2/callback'
provider: aliyun
oidc_issuer_url: 'https://oauth.aliyun.com/'
client_id: 'YOUR_CLIENT_ID'
client_secret: 'YOUR_CLIENT_SECRET'
scope: 'openid'
cookie_secret: 'YOUR_BASE64_COOKIE_SECRET'
service_name: 'aliyun.dns'
service_port: 443
match_type: whitelist
match_list:
 - match_rule_domain: 'foo.bar.com'
   match_rule_path: /foo
   match_rule_type: prefix
```

#### Access Service Page; Redirect if Not Logged In
![aliyun_login_1](https://gw.alicdn.com/imgextra/i1/O1CN01L379Uk1b2umAraylT_!!6000000003408-0-tps-3840-2160.jpg)
Directly login using a RAM user or click the main account login.
![aliyun_login_2](https://gw.alicdn.com/imgextra/i1/O1CN01pfdA3l27Dy2TL83NA_!!6000000007764-0-tps-3840-2160.jpg)

#### Successful Login Redirects to Service Page
![aliyun_result](https://gw.alicdn.com/imgextra/i3/O1CN015pGvi51eakt3pFS8Y_!!6000000003888-0-tps-3840-2160.jpg)

### OIDC Flow Diagram
<p align="center">
  <img src="https://gw.alicdn.com/imgextra/i3/O1CN01TJSh9c1VwR61Q2nek_!!6000000002717-55-tps-1807-2098.svg" alt="oidc_process" width="600" />
</p>

### OIDC Flow Analysis
#### User Not Logged In
1. Simulate user accessing the corresponding service API
   ```shell
   curl --url "foo.bar.com/headers"
   ```
2. Higress redirects to the OIDC Provider login page while carrying the client_id, response_type, scope, and other OIDC authentication parameters, setting a CSRF cookie to defend against CSRF attacks.
   ```shell
   curl --url "https://YOUR_AUTH0_DOMAIN/authorize"\
     --url-query "approval_prompt=force" \
     --url-query "client_id=YOUR_CLIENT_ID" \
     --url-query "redirect_uri=http%3A%2F%2Ffoo.bar.com%2Foauth2%2Fcallback" \
     --url-query "response_type=code" \
     --url-query "scope=openid+email+offline_access" \
     --url-query "state=YOUR_OAUTH_STATE%3A%2Ffoo" \
     --header "Set-Cookie: _oauth2_proxy_csrf=YOUR_CSRF_COOKIE; Path=/; Expires=Fri, 21 Jun 2024 08:06:20 GMT; HttpOnly"
   ```
3. Redirect to the login page.
![keycloak_login](https://gw.alicdn.com/imgextra/i4/O1CN01HLcl7r1boXwwnzGqA_!!6000000003512-0-tps-3840-2160.jpg)
4. The user enters the username and password to log in.
5. A redirect back to Higress occurs with the authorization code and state parameter for CSRF cookie validation.
   ```shell
   curl --url "http://foo.bar.com/oauth2/callback" \
     --url-query "state=YOUR_OAUTH_STATE%3A%2Ffoo" \
     --url-query "code=YOUR_AUTHORIZATION_CODE"
   ```
6. Verify that the encrypted state value stored in the CSRF cookie matches the state value in the URL parameters.
7. Use the authorization code to exchange for id_token and access_token.
   ```shell
   curl -X POST \
     --url "https://YOUR_AUTH0_DOMAIN/oauth/token" \
     --data "grant_type=authorization_code" \
     --data "client_id=YOUR_CLIENT_ID" \
     --data "client_secret=YOUR_CLIENT_SECRET" \
     --data "redirect_uri=http%3A%2F%2Ffoo.bar.com%2Foauth2%2Fcallback" \
     --data "code=YOUR_AUTHORIZATION_CODE" \
   ```
   The response will include id_token, access_token, and refresh_token for future token refresh.
   ```json
   {
       "access_token": "YOUR_ACCESS_TOKEN",
       "refresh_token": "YOUR_REFRESH_TOKEN",
       "id_token": "YOUR_ID_TOKEN",
       "scope": "openid email offline_access",
       "expires_in": 86400,
       "token_type": "Bearer"
   }
   ```
8. The obtained id_token, access_token, and refresh_token are encrypted and stored in the `_oauth2_proxy` cookie.
9. Redirect to the backend service accessed by the user and set cookies for subsequent user login state verification while clearing the `_oauth2_proxy_csrf` cookie.
   ```json
   "Set-Cookie": [
       "_oauth2_proxy_csrf=; Path=/; Expires=Mon, 24 Jun 2024 02:17:39 GMT; HttpOnly",
       "_oauth2_proxy=YOUR_SESSION_COOKIE; Path=/; Expires=Mon, 01 Jul 2024 03:17:36 GMT; HttpOnly"
   ]
   ```
10. Verify the existence of the cookie storing user's token information and check if it has expired.
11. Use the access token with the Authorization header to access the corresponding API.
    ```shell
    curl --url "foo.bar.com/headers"
      --header "Authorization: Bearer YOUR_ACCESS_TOKEN"
    ```
12. The backend service obtains user authorization information based on the access token and returns the corresponding HTTP response.
    ```json
    {
        "email": "******",
        "email_verified": false,
        "iss": "https://YOUR_AUTH0_DOMAIN/",
        "aud": "YOUR_CLIENT_ID",
        "iat": 1719198638,
        "exp": 1719234638,
        "sub": "YOUR_USER_ID",
        "sid": "YOUR_SESSION_ID"
    }
    ```

#### User Token Refresh
1. Simulate user accessing the corresponding service API.
```shell
curl --url "foo.bar.com/headers"
```
2. Verify the expiration time of the token.
3. If a refresh_token is detected in the cookie, access the corresponding interface to exchange a new id_token and access_token.
```shell
curl -X POST \
  --url "https://YOUR_AUTH0_DOMAIN/oauth/token" \
  --data "grant_type=refresh_token" \
  --data "client_id=YOUR_CLIENT_ID" \
  --data "client_secret=YOUR_CLIENT_SECRET" \
  --data "refresh_token=YOUR_REFRESH_TOKEN"
```
4. Access the corresponding API with the access token using the Authorization header.
5. The backend service obtains user authorization information based on the access token and returns the corresponding HTTP response.
