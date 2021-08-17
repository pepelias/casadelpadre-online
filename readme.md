# Plataforma de streamig rtmp
Implementaremos una plataforma de streaming para los servicios online de la Iglesia Casa del Padre de Tocopilla, Chile.
Nuestro backend es un pequeño servicio desarrollado en Go encargado de gestionar el chat y brindar la información del estado del stream al frontend.

> **Del servidor RTMP se encargará NGINX.**

## Frontend
El frontend es una app React con un repositorio aparte, contenido aquí: [https://github.com/pepelias/casadelpadre-online-frontend](https://github.com/pepelias/casadelpadre-online-frontend)

----

# Implementando un servicio de streaming con NGINX, RTMP Module, y servidor GO (Server y Websocket)

## Caracteristicas del Sistema
1. Ubuntu 18.04 (LTS)
2. Git (Preinstalado en droplet DigitalOceans)
3. Se requiere ffmpeg: `sudo apt install ffmpeg`

## Construir NGINX

Guía Basada en [https://www.nginx.com/blog/video-streaming-for-remote-learning-with-nginx/](https://www.nginx.com/blog/video-streaming-for-remote-learning-with-nginx/)

Construiremos NGINX incluyendo el módulo RTMP (y no olvidemos SSL).

### Instalar dependencias
```bash
$ sudo apt update
$ sudo apt install build-essential 
$ sudo apt install libpcre3-dev libssl-dev zlib1g-dev
```

### Clonar repositorios
```bash
$ git clone https://github.com/arut/nginx-rtmp-module.git
$ git clone https://github.com/nginx/nginx.git
```

### Construir NGINX
```bash
# Aquí se incluyen los módulos RTMP y SSL
$ cd nginx
$ ./auto/configure --add-module=../nginx-rtmp-module --with-http_ssl_module
$ make
$ sudo make install
```

### Administrar NGINX
```bash
# Iniciar NGINX
$ /usr/local/nginx/sbin/nginx

# Detener NGINX
$ /usr/local/nginx/sbin/nginx -s stop

# Archivo de configuración
$ /usr/local/nginx/conf/nginx.conf
```

## Configurar NGINX

Modificaremos el archivo `/usr/local/nginx/conf/nginx.conf` para lograr la configuración deseada.

El archivo `nginx.conf` contenido en `server_config/` tiene una configuración funcional y lista para ser utilizada, simplemente debe subirse al servidor tal y como está.

## Arquitectura implementada en nginx.conf

Para RTMP se configuran dos `applications`, una recibe el flujo y lo retransmite. La otra recibe la "re-transmisión" y sirve los flujos en HLS.

> Esto es necesario dado que se realiza una transcodificación, implementarlo en una sola app generaría un bucle infinito de flujos que causaría el colapso. 


### Aplication live

Su función es recibir el flujo principal y ejecutar la transcodificación. **Esta application NO IMPLEMENTA HLS, debe retransmitirse (hacer `push`) hacia la application hls para tener disponible el video en calidad máxima.**

Se necesita `ffmpeg` para transcodificar el video y obtener multiples calidades

```bash
$ sudo apt install ffmpeg
```

El comando `ffmpeg` dentro de `exec_push` contiene el comodin `$name`. Este es el nombre del stream, así se nombran los archivos.
**El `$name` es en verdad la clave de transmisión**

En las salidas especificadas en `ffmpeg`, le añadimos un `_[algo]` al `$name`, para diferenciar las diferentes calidades.

```conf
application live {
  live on;
  # Retransmitir máxima calidad
  push rtmp://localhost/hls/[Nombre del stream];
  # Transcodificar a menores calidades
  exec_push ffmpeg -i rtmp://localhost/live/$name rtmp://localhost/hls/$name_mid;
}
```

### Application hls

Esta `application` recibe los flujos transcodificados (o retransmitidos) e implementa HLS para que estén disponibles.
También notifica al servicio de backend (aplicación externa) cuando la transmisión comenzó o terminó.

```conf
application hls {
  live on;
  record off;

  hls_path [directorio temporal];

  ...[configurar HLS]

  # Notificar
  on_publish [dirección http para notificar inicio];
  on_publish_done [dirección http para notificar término];
}
```

### Servidor HTTP

Debe servir el mismo directorio configurado en `hls_path`.
La configuración apropiada de los headers puede encontrarse en el archivo `nginx.conf`


## Transmitir
Ya el servidor está preparado para recibir un video y servirlo en multiples calidades.

> Recordemos que la clave de transmisión para nginx es el `$name`

### Transmitir video
```rtmp://localhost/live/[clave]```

### Consumir video
```
# Calidad alta
http://localhost/video/[clave]/index.m3u8

# Calidad media
http://localhost/video/[clave]_mid/index.m3u8

# Calidad baja
http://localhost/video/[clave]_low/index.m3u8
```

## Servicio administrador (Backend, chat y websocket)

NGINX notificará a nuestro backend los eventos necesarios para que nuestra app esté enterada cuando comenzó y terminó una transmisión (Entre varios otros eventos.)

Para leer todos los eventos visite este link:
[https://github.com/arut/nginx-rtmp-module/wiki/Directives#notify](https://github.com/arut/nginx-rtmp-module/wiki/Directives#notify)

### Servidor GO
Nuestro backend puede ser tan complejo como queramos. En este caso implementaremos simplemente un chat y usaremos el websocket para avisar al cliente que un stream comenzó y terminó. El servicio está construido en GO su su configuración es minima y sencilla.

### Deploy del servidor.
Simplemente deben subirse los archivos `casadelpadre-online` y `configuration.json` al mismo directorio. En nuestro caso `/home`.

### Configuration.json
Debemos especificar las rutas y nombres según la configuración de nginx.

1. **qualities:** Especifica las rutas de los archivos m3u8
2. **streams:** Especifica los nombres que tomará cada transmisión

```json
"qualities": {
    "high": "http://localhost/video/miClave/index.m3u8",
    "mid": "http://localhost/video/miClave_mid/index.m3u8",
    "low": "http://localhost/video/miClave_mid/index.m3u8"
  },
"streams": {
  "high": "miClave",
  "mid": "miClave_mid",
  "low": "miClave_low"
}
```

## Comunicar NGINX con nuestro Backend GO:
Nuestra pequeña app GO espera la notificación de inicio de las tres calidades, solo después de eso envia el aviso al frontend. **Por eso es importante el apartado `streams` en `configuration.json`**

Se establecieron dos endpoints donde nginx debe notificar

### Notificar inicio
```POST /v1/streaming/on```

La app `hls` en `nginx.conf` debe especificar lo siguiente:
```conf
on_publish http://localhost:8080/v1/streaming/go
```

### Notificar término
```POST /v1/streaming/off```

La app `hls` en `nginx.conf` debe especificar lo siguiente:
```conf
on_publish_done http://localhost:8080/v1/streaming/off
```

## Crear nuestro servicio (daemon)
Crearemos un servicio para administrar de mejor manera nuesta aplicación GO

Debemos crear el archivo `streaming.service` en `/etc/systemd/system/`. **En este repo contiene el archivo `server_config/streaming.service` con el contenido necesario.**

### Administrar servicio
```bash
# Inicializar servicio
$ sudo systemctl start streaming
# Detener servicio
$ sudo systemctl stop streaming
# Saber estado servicio
$ sudo systemctl status streaming
# Habilitar servicio (iniciar al encender la maquina)
$ sudo systemctl enable streaming
```

## Instalar certificados SSL
Nuestro sistema necesita funcionar en HTTPS, esto implica configurar nginx y también nuestro servicio GOLANG. Para ello utilizaremos `certbot`

> **No usaremos la configuración para nginx, porque es una compilación nuestra. En su lugar configuraremos para "otro servidor" manualmente**

Guia basada en: [https://certbot.eff.org/lets-encrypt/ubuntubionic-other](https://certbot.eff.org/lets-encrypt/ubuntubionic-other)

### Actualizar snapd
```bash
$ sudo snap install core; sudo snap refresh core
```

### Preparar comando certbot
```bash
$ sudo ln -s /snap/bin/certbot /usr/bin/certbot
```

### Preparar comando certbot
```bash
$ sudo ln -s /snap/bin/certbot /usr/bin/certbot
```

### Generar certificado con el servidor corriendo
Para esto ya debe estar disponible el dominio y funcionando en nuestro servidor
```bash
$ sudo certbot certonly --webroot
```

### Seguir instrucciones de instalación
Al terminar este paso, la consola nos indicará la ruta donde se almacenaron nuestros certificados

### Configurar nginx
Ya está en el archivo `nginx.conf`, pero basicamente, aparte del servidor http normal (en el puerto 80). Debe haber otro en el puerto `443`, que indique que es `ssl` e incluya la ruta de estos archivos

```bash
listen 443 ssl;

ssl_certificate /etc/letsencrypt/live/iglesiacasadelpadre.cl/fullchain.pem;
ssl_certificate_key /etc/letsencrypt/live/iglesiacasadelpadre.cl/privkey.pem;
```

### Configurar servidor GO
En este caso solamente debemos indicar las rutas en el apartado `ssl` de `configuration.json`

```json
"ssl": {
  "cert": "/etc/letsencrypt/live/iglesiacasadelpadre.cl/fullchain.pem",
  "key": "/etc/letsencrypt/live/iglesiacasadelpadre.cl/privkey.pem"
}
```

> Obviamente se deben reiniciar ambos servidores

## Renovación automática
Cada vez que los certificados se renueven (cada tres meses), necesitamos que nuestros servicios se reinicien (para tomar los nuevos certificados). Para eso crearemos estos scripts que certbot correrá automáticamente.

```bash
$ sudo sh -c 'printf "#!/bin/sh\nservice streaming stop\n" > /etc/letsencrypt/renewal-hooks/pre/streaming.sh'
$ sudo sh -c 'printf "#!/bin/sh\nservice streaming start\n" > /etc/letsencrypt/renewal-hooks/post/streaming.sh'

$ sudo chmod 755 /etc/letsencrypt/renewal-hooks/pre/streaming.sh
$ sudo chmod 755 /etc/letsencrypt/renewal-hooks/post/streaming.sh

$ sudo sh -c 'printf "/usr/local/nginx/sbin/nginx -s stop\n" > /etc/letsencrypt/renewal-hooks/pre/nginx.sh'
$ sudo sh -c 'printf "/usr/local/nginx/sbin/nginx\n" > /etc/letsencrypt/renewal-hooks/post/nginx.sh'

$ sudo chmod 755 /etc/letsencrypt/renewal-hooks/pre/nginx.sh
$ sudo chmod 755 /etc/letsencrypt/renewal-hooks/post/nginx.sh
```

### Probar renovación
```bash
$ sudo certbot renew --dry-run
```