package ELLIO_Traefik_Middleware_Plugin

import (
	"net/http"
)

// blockPageHTML contains the HTML for the 403 Forbidden page
const blockPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>403 - Access Forbidden | ELLIO</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Montserrat:wght@300;400;500;600;700&display=swap" rel="stylesheet">
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        :root {
            --primary: #0094FF;
            --primary-light: #3AAFFF;
            --primary-dark: #0070CC;
            --bg-dark: #0A1628;
            --bg-darker: #040B14;
            --text-primary: #F8FAFC;
            --text-secondary: #94A3B8;
            --accent: #1E3A5F;
        }

        body {
            font-family: 'Montserrat', sans-serif;
            background: linear-gradient(135deg, var(--bg-darker) 0%, var(--bg-dark) 100%);
            color: var(--text-primary);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            overflow: hidden;
            position: relative;
        }

        .container {
            text-align: center;
            z-index: 10;
            position: relative;
            padding: 2rem;
            animation: fadeInUp 1s ease-out;
        }

        @keyframes fadeInUp {
            from {
                opacity: 0;
                transform: translateY(30px);
            }
            to {
                opacity: 1;
                transform: translateY(0);
            }
        }

        .logo-container {
            margin-bottom: 3rem;
            position: relative;
            display: inline-block;
        }

        .logo {
            width: 120px;
            height: auto;
            filter: drop-shadow(0 0 20px rgba(0, 148, 255, 1)) drop-shadow(0 0 35px rgba(0, 148, 255, 0.5));
            position: relative;
        }


        .error-code {
            font-size: 6rem;
            font-weight: 700;
            background: linear-gradient(135deg, var(--primary) 0%, var(--primary-light) 100%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
            margin-bottom: 1rem;
            letter-spacing: -0.02em;
            position: relative;
            animation: fadeInScale 1s ease-out;
        }

        @keyframes fadeInScale {
            from {
                opacity: 0;
                transform: scale(0.9);
            }
            to {
                opacity: 1;
                transform: scale(1);
            }
        }

        h1 {
            font-size: 2rem;
            font-weight: 600;
            margin-bottom: 1rem;
            letter-spacing: -0.01em;
        }

        .message {
            font-size: 1.125rem;
            color: var(--text-secondary);
            margin-bottom: 2rem;
            line-height: 1.6;
            max-width: 500px;
            margin-left: auto;
            margin-right: auto;
        }

        .lock-animation {
            width: 60px;
            height: 60px;
            margin: 2rem auto;
            position: relative;
        }

        .lock-body {
            width: 40px;
            height: 30px;
            background: linear-gradient(135deg, var(--primary) 0%, var(--primary-dark) 100%);
            border-radius: 4px;
            position: absolute;
            bottom: 0;
            left: 50%;
            transform: translateX(-50%);
            box-shadow: 0 4px 20px rgba(0, 148, 255, 0.4);
        }

        .lock-shackle {
            width: 24px;
            height: 24px;
            border: 4px solid var(--primary);
            border-bottom: none;
            border-radius: 12px 12px 0 0;
            position: absolute;
            top: 0;
            left: 50%;
            transform: translateX(-50%);
            animation: lockShackle 4s ease-in-out infinite;
        }

        @keyframes lockShackle {
            0%, 45%, 100% {
                transform: translateX(-50%) rotate(0deg);
            }
            50%, 95% {
                transform: translateX(-50%) rotate(-10deg) translateX(-2px);
            }
        }

        .protection-footer {
            position: absolute;
            bottom: 2rem;
            left: 50%;
            transform: translateX(-50%);
            font-size: 0.875rem;
            color: var(--text-secondary);
            animation: fadeIn 1s ease-out 0.8s both;
        }

        @keyframes fadeIn {
            from {
                opacity: 0;
            }
            to {
                opacity: 1;
            }
        }

        .protection-footer span {
            margin-right: 0.25rem;
        }

        .protection-footer a {
            color: var(--primary);
            text-decoration: none;
            font-weight: 500;
            transition: all 0.3s ease;
            position: relative;
        }

        .protection-footer a::after {
            content: '';
            position: absolute;
            bottom: -2px;
            left: 0;
            width: 0;
            height: 1px;
            background: var(--primary);
            transition: width 0.3s ease;
        }

        .protection-footer a:hover {
            color: var(--primary-light);
            text-shadow: 0 0 10px rgba(0, 148, 255, 0.5);
        }

        .protection-footer a:hover::after {
            width: 100%;
        }

        @media (max-width: 768px) {
            .error-code {
                font-size: 4rem;
            }

            h1 {
                font-size: 1.5rem;
            }

            .message {
                font-size: 1rem;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="logo-container">
            <img src="https://cdn.ellio.tech/logo/ELLIO_dark.png" alt="ELLIO Logo" class="logo">
        </div>

        <div class="error-code">403</div>

        <h1>Forbidden</h1>

        <div class="lock-animation">
            <div class="lock-shackle"></div>
            <div class="lock-body"></div>
        </div>

        <p class="message">
            Access to this resource is denied.
        </p>

        <div class="protection-footer">
            <span>Protection by</span>
            <a href="https://ellio.tech" target="_blank" rel="noopener noreferrer">ELLIO</a>
        </div>
    </div>

</body>
</html>`

// ServeBlockPage serves the HTML 403 block page
func ServeBlockPage(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(blockPageHTML))
}
