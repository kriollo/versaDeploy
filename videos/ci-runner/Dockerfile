# The self-hosted GitHub Actions runner of episode 7: GitHub's runner image plus the
# toolchains the demo project builds with (PHP + Composer, Node + pnpm). It runs with
# --network host, so it reaches the video's "VPS" (127.0.0.1:2299) like the laptop does.
FROM ghcr.io/actions/actions-runner:latest
USER root
RUN apt-get update && apt-get install -y --no-install-recommends \
        php-cli php-xml php-mbstring php-curl php-zip php-intl php-pgsql unzip git curl ca-certificates openssh-client \
    && curl -fsSL https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer \
    && curl -fsSL https://deb.nodesource.com/setup_22.x | bash - && apt-get install -y nodejs \
    && npm install -g pnpm@10 \
    && rm -rf /var/lib/apt/lists/*
USER runner
