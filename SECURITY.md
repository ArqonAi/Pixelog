# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.0.x   | :white_check_mark: |

## Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability, please:

1. **Do NOT** create a public GitHub issue
2. Email us at security@pixelog.dev (or create a private security advisory)
3. Include:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if available)

## Security Measures

### Backend Security
- Input validation and sanitization
- File upload size limits (100MB per file)
- Secure temporary file handling
- No shell injection vulnerabilities
- CORS properly configured
- Health check endpoints

### Frontend Security
- Content Security Policy headers
- XSS protection via React's built-in escaping
- Secure file upload handling
- No sensitive data in localStorage
- HTTPS enforcement in production

### Infrastructure Security
- Docker containers run as non-root user
- Minimal attack surface (Alpine Linux base)
- No secrets in code or containers
- Environment variable configuration
- Proper file permissions