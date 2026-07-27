FROM mwader/static-ffmpeg:8.0 AS ffmpeg8

FROM php:8.0-apache

RUN set -eux; \
    sed -i 's|http://deb.debian.org/debian|http://mirrors.aliyun.com/debian|g' /etc/apt/sources.list; \
    printf 'Acquire::Retries "5";\nAcquire::http::Timeout "60";\n' > /etc/apt/apt.conf.d/99local-retries; \
    apt-get update \
    && apt-get install -y --no-install-recommends \
        default-mysql-client \
        git \
        libcurl4-openssl-dev \
        libfreetype6-dev \
        libjpeg62-turbo-dev \
        libonig-dev \
        libpng-dev \
        libzip-dev \
        python3 \
        tzdata \
        unzip \
        zip \
    && docker-php-ext-configure gd --with-freetype --with-jpeg \
    && docker-php-ext-install -j"$(nproc)" \
        bcmath \
        curl \
        gd \
        mbstring \
        mysqli \
        pdo_mysql \
        sockets \
        zip \
    && pecl install redis \
    && docker-php-ext-enable redis \
    && a2enmod rewrite headers proxy proxy_http proxy_wstunnel \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

COPY --from=ffmpeg8 /ffmpeg /usr/local/bin/ffmpeg
COPY --from=ffmpeg8 /ffprobe /usr/local/bin/ffprobe
RUN chmod +x /usr/local/bin/ffmpeg /usr/local/bin/ffprobe

COPY apache-vhost.conf /etc/apache2/sites-available/000-default.conf
COPY php.ini /usr/local/etc/php/conf.d/claw-local.ini

ENV TZ=Asia/Shanghai
