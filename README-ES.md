# KubeTray & Sync

Script en Bash para sincronizar automáticamente los contextos de clusters **Google Kubernetes Engine (GKE)** de todos los proyectos de tu organización o cuenta de Google Cloud Platform (GCP) en tu archivo local `kubeconfig`.

---

## 🚀 Características

- **Autenticación Interactiva**: Detecta automáticamente si tienes una sesión activa en `gcloud`. Si no la tienes, solicita confirmación por terminal para abrir el navegador e iniciar sesión (`gcloud auth login`).
- **Filtro de Proyectos de Sistema**: Ignora automáticamente todos los proyectos con prefijo `sys-*`.
- **Prevención de Duplicados**: Verifica si el contexto (`gke_<PROYECTO>_<UBICACIÓN>_<CLUSTER>`) ya existe en tu archivo `kubeconfig` antes de solicitar las credenciales, evitando reescrituras innecesarias.
- **Manejo Seguro de Errores**: Si un proyecto no tiene la API de Kubernetes Engine habilitada o no se poseen permisos, continúa con el siguiente proyecto sin interrumpir el proceso general.
- **Modo Simulación (Dry-Run)**: Permite previsualizar los clusters que se añadirían sin modificar `kubeconfig`.
- **Resumen Estadístico**: Al finalizar, muestra métricas con la cantidad de proyectos escaneados, clusters encontrados, agregados y omitidos.

---

## 📋 Requisitos Previos

- **[Google Cloud SDK (`gcloud`)](https://cloud.google.com/sdk/docs/install)**
- **[`kubectl`](https://kubernetes.io/docs/tasks/tools/)**
- **Bash 4.0+**

---

## 🛠️ Instalación y Uso

1. Otorga permisos de ejecución al script:
   ```bash
   chmod +x sync-gke.sh
   ```

2. Ejecuta el script:
   ```bash
   ./sync-gke.sh
   ```

---

## ⚙️ Opciones y Parámetros

| Opción | Descripción |
| :--- | :--- |
| `-d`, `--dry-run` | Muestra qué clusters se añadirían sin realizar cambios en `kubeconfig`. |
| `-f`, `--force` | Fuerza la obtención de credenciales incluso si el contexto ya existe. |
| `-i`, `--internal-ip` | Agrega los clusters utilizando su endpoint de IP interna (`--internal-ip`). |
| `-v`, `--verbose` | Muestra mensajes detallados durante la ejecución. |
| `-h`, `--help` | Muestra el menú de ayuda. |

### Ejemplos

**Simulación sin aplicar cambios:**
```bash
./sync-gke.sh --dry-run
```

**Sincronización estándar:**
```bash
./sync-gke.sh
```

**Sincronización forzando actualización de credenciales:**
```bash
./sync-gke.sh --force
```

---

## 🖱️ App de bandeja: `kubetray`

Icono de bandeja para cambiar el contexto actual de kubeconfig, hecho
igual que `browser-switcher`: sin GTK, el icono y el menú hablan
directamente con las interfaces D-Bus StatusNotifierItem y
com.canonical.dbusmenu de freedesktop, así que las únicas dependencias
en tiempo de ejecución son D-Bus y `kubectl`.

- El menú lista todos los contextos de `kubectl config get-contexts`,
  con una marca de radio en el actual. Los contextos de GKE
  (`gke_<proyecto>_<ubicación>_<cluster>`) se agrupan bajo su proyecto
  y se muestran como `cluster · ubicación`; el resto va bajo "Other".
- Al hacer clic en uno se ejecuta `kubectl config use-context <nombre>`.
- Los archivos de kubeconfig (`$KUBECONFIG`, o `~/.kube/config`) se
  revisan cada 2s, así que un `kubectl config use-context` desde una
  terminal, o clusters nuevos añadidos por `sync-gke.sh`, aparecen sin
  reiniciar la app.
- El tooltip muestra el cluster y proyecto actuales. El icono se pone
  rojo cuando el nombre del contexto actual contiene `prd`, `prod` o
  `production` como palabra separada.
- Si el plugin de credenciales del contexto actual necesita una sesión
  de nube que no está activa, aparece un aviso con el comando para
  arreglarlo debajo de los contextos (GCP: `gcloud auth print-access-token`;
  AWS: `aws sts get-caller-identity` con el perfil del plugin; Azure:
  `az account get-access-token`, solo en el modo `azurecli` de kubelogin).
  Se vuelve a comprobar cada minuto.
- "Launch at login" activa/desactiva `~/.config/autostart/kubetray.desktop`.

`kubectl` se busca en el `PATH`, y si no está, en `~/google-cloud-sdk/bin`,
`~/.local/bin`, `/usr/local/bin` y `/usr/bin` (las sesiones de autostart
no suelen heredar el `PATH` de tu shell). Usa `$KUBECTL` para forzar una ruta.

Necesita un StatusNotifierHost para mostrar el icono, p. ej. el módulo
`tray` de waybar, o `snixembed` si tu barra no soporta StatusNotifierItem.

```bash
make build                # genera ./kubetray
sudo make install         # instala en /usr/local/bin (PREFIX=/usr para cambiarlo)
sudo make uninstall
```

---

## 📄 Licencia

MIT
