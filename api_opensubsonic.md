API Reference
Common API documentation.
Opensubsonic:

    ClarificationChange

Parameters

Please note that all methods take the following parameters:
Parameter	Req.	OpenS.	Default	Comment
u	Yes**			The username.
p	Yes*			The password, either in clear text or hex-encoded with a “enc:” prefix. Since 1.13.0 this should only be used for testing purposes.
t	Yes*			(Since 1.13.0) The authentication token computed as md5(password + salt). See below for details.
s	Yes*			(Since 1.13.0) A random string (“salt”) used as input for computing the password hash. See below for details.
apiKey	Yes**	Yes		[OS] An API key used for authentication
v	Yes			The protocol version implemented by the client, i.e., the version of the subsonic-rest-api.xsd schema used (see below).
c	Yes			A unique string identifying the client application.
f	No		xml	Request data to be returned in this format. Supported values are “xml”, “json” (since 1.4.0) and “jsonp” (since 1.6.0). If using jsonp, specify name of javascript callback function using a callback parameter.

*) Either p or both t and s must be specified.

**) If apiKey is specified, then none of p, t, s, nor u can be specified.

Remember to URL encode the request parameters. All methods (except those that return binary data) returns XML documents conforming to the subsonic-rest-api.xsd schema. The XML documents are encoded with UTF-8.
POST support

OpenSubsonic add official support for application/x-www-form-urlencoded POST to pass the argument.

Check that the server support the HTTP form POST extension before using it.

The arguments can then be passed in the POST body (Do not forget to URL encode both the keys and values), this allows to overcome the URL size limits when passing many parameters for playlists for example.
curl -v -X POST -H 'Content-Type: application/x-www-form-urlencoded' 'http://your-server/rest/ping.view' --data 'c=AwesomeClientName&v=1.12.0&f=json&u=joe&p=sesame'
Authentication

If you are targeting API version 1.12.0 or earlier, authentication is performed by sending the password as clear text or hex-encoded. Examples:
http://your-server/rest/ping.view?u=joe&p=sesame&v=1.12.0&c=AwesomeClientName&f=json
http://your-server/rest/ping.view?u=joe&p=enc:736573616d65&v=1.12.0&c=AwesomeClientName&f=json

Starting with API version 1.13.0, the recommended authentication scheme is to send an authentication token, calculated as a one-way salted hash of the password.

This involves two steps:

    For each REST call, generate a random string called the salt. Send this as parameter s. Use a salt length of at least six characters.
    Calculate the authentication token as follows: token = md5(password + salt). The md5() function takes a string and returns the 32-byte ASCII hexadecimal representation of the MD5 hash, using lower case characters for the hex values. The ‘+’ operator represents concatenation of the two strings. Treat the strings as UTF-8 encoded when calculating the hash. Send the result as parameter t.

For example: if the password is sesame and the random salt is c19b2d, then token = md5(“sesamec19b2d”) = 26719a1196d2a940705a59634eb18eab. The corresponding request URL then becomes:
http://your-server/rest/ping.view?u=joe&t=26719a1196d2a940705a59634eb18eab&s=c19b2d&v=1.13.0&c=AwesomeClientName&f=json
OpenSubsonic

A new authentication mechanism is available.

See API Key authentication

For servers that implement API Key authentication, the recommended authentication is to use an API key. This is a token generated from the Subsonic server. It must be passed in in as apiKey=<API key>, and the u parameter must not be provided. Note that u/p may still be used by servers which are backed by LDAP/PAM/other authentication.
http://your-server/rest/ping.view?u=joe&apiKey=43504ab81e2bfae1a7691fe3fc738fdf55ada2757e36f14bcf13d&v=1.16.1&c=AwesomeClientName&f=json
Subsonic-response

All API endpoint unless noted otherwise returns a subsonic-response that indicate the result of the command and give some information about the server.

{
  "subsonic-response": {
    "status": "ok",
    "version": "1.16.1",
    "type": "AwesomeServerName",
    "serverVersion": "0.1.3 (tag)",
    "openSubsonic": true
  }
}

See: subsonic-response for the field details.
OpenSubsonic

New fields are added:

See subsonic-response
Error handling

If a method fails it will return an error code and message in an error element. In addition, the status attribute of the subsonic-response root element will be set to failed instead of ok. For example:

{
  "subsonic-response": {
    "status": "failed",
    "version": "1.16.1",
    "type": "AwesomeServerName",
    "serverVersion": "0.1.3 (tag)",
    "openSubsonic": true,
    "error": {
      "code": 40,
      "message": "Wrong username or password"
    }
  }
}

Field	Type	Req.	OpenS.	Details
error	error	Yes		The error details.
code	int	Yes		The error code.
message	string	No		A human readable error message.

The following error codes are defined:
Code	Description
0	A generic error.
10	Required parameter is missing.
20	Incompatible Subsonic REST protocol version. Client must upgrade.
30	Incompatible Subsonic REST protocol version. Server must upgrade.
40	Wrong username or password.
41	Token authentication not supported for LDAP users.
42	Provided authentication mechanism not supported.
43	Multiple conflicting authentication mechanisms provided.
44	Invalid API key.
50	User is not authorized for the given operation.
60	The trial period for the Subsonic server is over. Please upgrade to Subsonic Premium. Visit subsonic.org for details.
70	The requested data was not found.
OpenSubsonic

Servers must return error 41 if they do not support token-based authentication, 42 if they do not support any other authentication mechanism (password-based and/or API key-based), and 43 if multiple conflicting authentication parameters are passed in at the same time.

Note that even though the error text for 41 is Token authentication not supported for LDAP users., it actually implies that Token authentication is not supported for any reason.

To indicate differences between cases (where LDAP is used, versus no LDAP), servers may use the new helpUrl field. New fields are added, see error


------------------



