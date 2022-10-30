# PyMetrados v2.1.0 - Calculadora Profesional de Metrados

Una aplicación de escritorio moderna para cálculos de metrados de construcción con sistema de actualizaciones automáticas y analíticas simplificadas con **Mixpanel**.

## 🚀 Características Principales

### ✨ Interfaz Moderna en Español
- **Diseño profesional** con tarjetas elegantes y efectos hover
- **Interfaz completamente en español** para usuarios hispanohablantes
- **Botón de procesamiento prominente** que cambia de color según el estado
- **Mensajes informativos** que guían al usuario paso a paso
- **Retroalimentación visual** clara en cada acción

### 🔄 Sistema de Actualizaciones Automáticas
- **Verificación automática** al iniciar la aplicación
- **Descarga e instalación** automática de nuevas versiones
- **Notificaciones elegantes** cuando hay actualizaciones disponibles
- **GitHub Releases** como servidor de actualización

### 📊 Analíticas Simplificadas con Mixpanel
- **Eventos básicos** y útiles sin sobrecarga de datos
- **Configuración simple** con solo el token de Mixpanel
- **Eventos en español** para mejor comprensión
- **Privacidad respetada** con datos mínimos necesarios

## 📋 Requisitos del Sistema

### Dependencias Principales
```
Python >= 3.8
tkinter (incluido con Python)
Pillow >= 8.3.0
mixpanel >= 4.10.0
requests >= 2.28.0
```

## 🛠️ Instalación Rápida

### 1. Instalar Dependencias
```bash
pip install -r requirements.txt
```

### 2. Configurar Mixpanel (Opcional)
Edita `config.json`:
```json
{
    "analytics": {
        "mixpanel_token": "TU_TOKEN_DE_MIXPANEL",
        "enabled": true
    }
}
```

### 3. Ejecutar Aplicación
```bash
python pymetrados.py
```

## 🎮 Uso de la Aplicación

### Proceso Simple en 4 Pasos:

1. **📋 Seleccionar Archivo de Configuración**
   - Haz clic en "Examinar" en la primera tarjeta
   - Selecciona tu archivo Excel de configuración (.xlsx)
   
2. **📊 Seleccionar Archivo de Planilla**
   - Haz clic en "Examinar" en la segunda tarjeta
   - Selecciona tu archivo Excel de planilla (.xlsx)
   
3. **💾 Seleccionar Carpeta de Exportación**
   - Haz clic en "Seleccionar" en la tercera tarjeta
   - Elige dónde guardar el archivo resultado
   
4. **🚀 Iniciar Procesamiento**
   - El botón se volverá **verde** automáticamente
   - Mensaje: "✅ ¡Todo listo! Haga clic para iniciar el procesamiento"
   - Haz clic en "🚀 INICIAR PROCESAMIENTO"

### Estados del Botón de Procesamiento:

- **🔴 Gris**: Faltan archivos por seleccionar
  - Mensaje: "⚠️ Falta seleccionar: archivo de configuración, archivo de planilla"
  
- **🟢 Verde**: Todo listo para procesar
  - Mensaje: "✅ ¡Todo listo! Haga clic para iniciar el procesamiento"

## 📊 Eventos de Mixpanel (Simplificados)

La aplicación envía solo los eventos esenciales:

```javascript
// Al abrir la aplicación
App_Iniciado: {
    user_id: "abc12345",
    version: "2.1.0",
    sistema: "Windows"
}

// Al seleccionar archivos
Archivo_Seleccionado: {
    user_id: "abc12345",
    tipo: "configuracion" | "planilla"
}

// Al seleccionar carpeta
Carpeta_Seleccionada: {
    user_id: "abc12345"
}

// Al iniciar procesamiento
Procesamiento_Iniciado: {
    user_id: "abc12345",
    hora: "2024-01-15T10:30:00"
}

// Al completar exitosamente
Procesamiento_Exitoso: {
    user_id: "abc12345",
    tiempo_segundos: 12.4
}

// En caso de error
Procesamiento_Error: {
    user_id: "abc12345",
    error: "mensaje_del_error"
}

// Al cerrar aplicación
App_Cerrado: {
    user_id: "abc12345"
}
```

## 🔧 Configuración

### Archivo config.json Simplificado:
```json
{
    "app": {
        "name": "PyMetrados",
        "version": "2.1.0"
    },
    "update_server": {
        "github_repo": "yourusername/pymetrados",
        "api_url": "https://api.github.com/repos/yourusername/pymetrados/releases/latest"
    },
    "analytics": {
        "mixpanel_token": "tu_token_de_mixpanel",
        "enabled": true
    },
    "features": {
        "update_notifications": true
    }
}
```

### Deshabilitar Analíticas:
```json
{
    "analytics": {
        "enabled": false
    }
}
```

### Deshabilitar Actualizaciones:
```json
{
    "features": {
        "update_notifications": false
    }
}
```

## 🎯 Beneficios de la Versión Simplificada

### Para el Usuario:
- ✅ **Interfaz más clara** y fácil de usar
- ✅ **Menos distracciones** - solo lo esencial
- ✅ **Respuesta rápida** sin procesos innecesarios
- ✅ **Mensajes claros** en español
- ✅ **Proceso guiado** paso a paso

### Para el Desarrollador:
- ✅ **Código 70% más limpio** y mantenible
- ✅ **Analíticas útiles** sin ruido
- ✅ **Debugging más fácil** con menos complejidad
- ✅ **Actualizaciones simples** del código

## 🎨 Características de la Interfaz

### Diseño Moderno:
- **Tarjetas elegantes** con efectos hover
- **Paleta de colores profesional** (azul primario, verde éxito)
- **Tipografía Segoe UI** para consistencia con Windows
- **Iconos emotivos** para mejor UX (📋, 📊, 💾, 🚀)

### Retroalimentación Visual:
- **Estados claros** de cada archivo seleccionado
- **Botón dinámico** que cambia según disponibilidad
- **Mensajes informativos** que guían al usuario
- **Barra de progreso** durante el procesamiento

## 🔄 Sistema de Actualizaciones

### Flujo Automático:
1. **Al iniciar** → Verificar actualizaciones en GitHub
2. **Si hay nueva versión** → Mostrar notificación elegante
3. **Usuario acepta** → Descargar con progreso visual
4. **Instalación** → Un clic y cierre automático
5. **Tracking** → Eventos en Mixpanel para analíticas

## 🐛 Solución de Problemas

### El botón no aparece verde:
- Verifica que hayas seleccionado **todos** los archivos
- El mensaje debe decir "✅ ¡Todo listo!"

### No se muestran analíticas:
- Verifica tu token de Mixpanel en `config.json`
- Asegúrate de que `enabled: true`

### Error al procesar:
- Verifica que los archivos Excel sean válidos
- Asegúrate de tener permisos de escritura en la carpeta

## 📞 Contacto

**Angel Huayas** - www.angelhuayas.com

---

**¡Tu PyMetrados ahora es más simple, claro y efectivo!** 🚀✨ 