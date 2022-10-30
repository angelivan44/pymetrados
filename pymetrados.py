import tkinter as tk
from tkinter import ttk, filedialog, messagebox
from PIL import Image, ImageTk
import os
import scrip_metrados
import sys
import json
import requests
import platform
import psutil
import uuid
import threading
import time
from datetime import datetime
import subprocess
import tempfile
import shutil
from mixpanel import Mixpanel

# Configuration
APP_VERSION = "2.1.0"
UPDATE_SERVER_URL = "https://api.github.com/repos/yourusername/pymetrados/releases/latest"  # Cambiar por tu repo

# Initialize Mixpanel - Simple configuration
def load_config():
    """Carga configuración desde config.json"""
    try:
        with open('config.json', 'r') as f:
            return json.load(f)
    except:
        return {
            "analytics": {
                "mixpanel_token": "3f89c70f58f582dfc898380a1699e843",
                "enabled": True
            },
            "features": {
                "update_notifications": True
            }
        }

# Cargar configuración
config = load_config()

# Inicializar Mixpanel (simplificado)
if config.get("analytics", {}).get("enabled", True):
    try:
        mp = Mixpanel(config["analytics"]["mixpanel_token"])
        ANALYTICS_ENABLED = True
    except:
        mp = None
        ANALYTICS_ENABLED = False
else:
    mp = None
    ANALYTICS_ENABLED = False

def track_event(event_name, properties=None):
    """Función simple para enviar eventos a Mixpanel"""
    if ANALYTICS_ENABLED and mp:
        try:
            user_id = properties.get('user_id', 'anonymous') if properties else 'anonymous'
            mp.track(user_id, event_name, properties or {})
        except:
            pass

class UpdateManager:
    """Clase simplificada para manejar actualizaciones"""
    
    def __init__(self, current_version=APP_VERSION):
        self.current_version = current_version
        self.update_available = False
        self.latest_version = None
        self.download_url = None
        
    def check_for_updates(self):
        """Verifica actualizaciones"""
        try:
            print(f"🔍 Verificando actualizaciones... (Actual: v{self.current_version})")
            
            response = requests.get(UPDATE_SERVER_URL, timeout=10)
            if response.status_code == 200:
                release_data = response.json()
                latest_version = release_data['tag_name'].replace('v', '')
                
                if self.is_newer_version(latest_version, self.current_version):
                    self.update_available = True
                    self.latest_version = latest_version
                    self.download_url = release_data['assets'][0]['browser_download_url'] if release_data['assets'] else None
                    
                    track_event('Actualización_Disponible', {
                        'version_actual': self.current_version,
                        'version_nueva': latest_version
                    })
                    
                    return True
            return False
        except Exception as e:
            print(f"❌ Error verificando actualizaciones: {e}")
            return False
    
    def is_newer_version(self, latest, current):
        """Compara versiones"""
        try:
            latest_parts = [int(x) for x in latest.split('.')]
            current_parts = [int(x) for x in current.split('.')]
            
            max_len = max(len(latest_parts), len(current_parts))
            latest_parts.extend([0] * (max_len - len(latest_parts)))
            current_parts.extend([0] * (max_len - len(current_parts)))
            
            return latest_parts > current_parts
        except:
            return False
    
    def download_update(self, progress_callback=None):
        """Descarga actualización"""
        if not self.download_url:
            return False
        
        try:
            response = requests.get(self.download_url, stream=True)
            total_size = int(response.headers.get('content-length', 0))
            
            temp_file = tempfile.NamedTemporaryFile(delete=False, suffix='.exe')
            downloaded = 0
            
            for chunk in response.iter_content(chunk_size=8192):
                if chunk:
                    temp_file.write(chunk)
                    downloaded += len(chunk)
                    
                    if progress_callback and total_size > 0:
                        progress = (downloaded / total_size) * 100
                        progress_callback(progress)
            
            temp_file.close()
            return temp_file.name
        except Exception as e:
            print(f"❌ Error descargando: {e}")
            return None

class PyMetradosApp:
    def __init__(self):
        # Variables básicas
        self.session_id = str(uuid.uuid4())[:8]
        self.update_manager = UpdateManager()
        
        # Configurar ventana
        self.window = tk.Tk()
        self.setup_window()
        self.setup_styles()
        self.create_variables()
        self.create_widgets()
        
        # Analytics simple
        track_event('App_Iniciado', {
            'user_id': self.session_id,
            'version': APP_VERSION,
            'sistema': platform.system(),
            'nombre_dispositivo': platform.node() or os.environ.get('COMPUTERNAME', os.environ.get('HOSTNAME', 'desconocido'))
        })
        
        # Verificar actualizaciones (si está habilitado)
        if config.get("features", {}).get("update_notifications", True):
            self.check_updates_on_startup()
        
    def check_updates_on_startup(self):
        """Verifica actualizaciones al iniciar"""
        def check_updates():
            time.sleep(1)
            if self.update_manager.check_for_updates():
                self.window.after(0, self.show_update_notification)
        
        threading.Thread(target=check_updates, daemon=True).start()
    
    def show_update_notification(self):
        """Muestra notificación de actualización"""
        update_window = tk.Toplevel(self.window)
        update_window.title("Nueva Actualización Disponible")
        update_window.geometry("450x280")
        update_window.configure(bg='white')
        update_window.resizable(False, False)
        
        # Centrar ventana
        update_window.transient(self.window)
        update_window.grab_set()
        
        main_frame = tk.Frame(update_window, bg='white', padx=30, pady=20)
        main_frame.pack(fill='both', expand=True)
        
        # Contenido
        icon_label = tk.Label(main_frame, text="🔄", font=('Segoe UI', 32), bg='white')
        icon_label.pack(pady=(0, 15))
        
        title_label = tk.Label(main_frame, text="Nueva Actualización Disponible", 
                              font=('Segoe UI', 16, 'bold'), bg='white', fg=self.colors['primary'])
        title_label.pack()
        
        version_label = tk.Label(main_frame, 
                                text=f"PyMetrados v{self.update_manager.latest_version}\n(Versión actual: v{APP_VERSION})",
                                font=('Segoe UI', 11), bg='white', fg='#666')
        version_label.pack(pady=(10, 5))
        
        info_label = tk.Label(main_frame, 
                             text="Se ha detectado una nueva versión.\n¿Deseas descargar e instalar ahora?",
                             font=('Segoe UI', 10), bg='white', fg='#444')
        info_label.pack(pady=10)
        
        # Botones
        button_frame = tk.Frame(main_frame, bg='white')
        button_frame.pack(pady=20)
        
        update_btn = tk.Button(button_frame, text="🚀 Actualizar Ahora",
                              font=('Segoe UI', 11, 'bold'),
                              bg=self.colors['success'], fg='white',
                              border=0, padx=25, pady=10,
                              cursor='hand2',
                              command=lambda: self.download_and_install_update(update_window))
        update_btn.pack(side='left', padx=(0, 15))
        
        later_btn = tk.Button(button_frame, text="Más Tarde",
                             font=('Segoe UI', 10),
                             bg='#6c757d', fg='white',
                             border=0, padx=20, pady=10,
                             cursor='hand2',
                             command=update_window.destroy)
        later_btn.pack(side='left')
    
    def download_and_install_update(self, update_window):
        """Descarga e instala actualización"""
        update_window.destroy()
        
        track_event('Descarga_Actualización', {
            'user_id': self.session_id,
            'version_objetivo': self.update_manager.latest_version
        })
        
        # Ventana de progreso
        progress_window = tk.Toplevel(self.window)
        progress_window.title("Descargando Actualización")
        progress_window.geometry("400x180")
        progress_window.configure(bg='white')
        progress_window.resizable(False, False)
        
        progress_frame = tk.Frame(progress_window, bg='white', padx=30, pady=25)
        progress_frame.pack(fill='both', expand=True)
        
        status_label = tk.Label(progress_frame, text="Descargando actualización...",
                               font=('Segoe UI', 13, 'bold'), bg='white', fg=self.colors['primary'])
        status_label.pack(pady=(0, 10))
        
        progress_bar = ttk.Progressbar(progress_frame, length=300, mode='determinate')
        progress_bar.pack(pady=10)
        
        percent_label = tk.Label(progress_frame, text="0%",
                                font=('Segoe UI', 10), bg='white', fg='#666')
        percent_label.pack(pady=(5, 0))
        
        def update_progress(progress):
            progress_bar['value'] = progress
            percent_label.config(text=f"{progress:.1f}%")
            progress_window.update()
        
        def download_thread():
            file_path = self.update_manager.download_update(update_progress)
            if file_path:
                self.window.after(0, lambda: self.install_update(file_path, progress_window))
            else:
                self.window.after(0, lambda: self.show_update_error(progress_window))
        
        threading.Thread(target=download_thread, daemon=True).start()
    
    def install_update(self, file_path, progress_window):
        """Instala actualización"""
        progress_window.destroy()
        
        result = messagebox.askyesno("Actualización Lista", 
                                   f"PyMetrados v{self.update_manager.latest_version} se ha descargado correctamente.\n\n"
                                   "¿Desea instalar ahora? La aplicación se cerrará automáticamente.")
        
        if result:
            try:
                subprocess.Popen([file_path])
                self.window.quit()
            except Exception as e:
                messagebox.showerror("Error", f"Error al instalar actualización:\n{e}")
    
    def show_update_error(self, progress_window):
        """Muestra error de actualización"""
        progress_window.destroy()
        messagebox.showerror("Error de Descarga", 
                           "No se pudo descargar la actualización.\n\n"
                           "Verifique su conexión a internet e intente más tarde.")
        
    def setup_window(self):
        """Configurar ventana principal"""
        self.window.title(f'PyMetrados v{APP_VERSION} - Calculadora Profesional de Metrados')
        self.window.geometry("900x750")  # Más alto
        self.window.resizable(False, False)  # Dimensiones fijas
        self.window.configure(bg='#f0f0f0')
        
        # Centrar ventana
        self.window.update_idletasks()
        x = (self.window.winfo_screenwidth() // 2) - (900 // 2)
        y = (self.window.winfo_screenheight() // 2) - (750 // 2)
        self.window.geometry(f"900x750+{x}+{y}")
    def setup_styles(self):
        """Configurar estilos"""
        self.style = ttk.Style()
        self.style.theme_use('clam')
        
        # Paleta de colores
        self.colors = {
            'primary': '#2E86AB',
            'secondary': '#A23B72',
            'accent': '#F18F01',
            'success': '#28a745',
            'warning': '#ffc107',
            'danger': '#dc3545',
            'light': '#f8f9fa',
            'dark': '#343a40',
            'white': '#ffffff'
        }
        
        self.style.configure('Title.TLabel', 
                           font=('Segoe UI', 28, 'bold'),
                           foreground=self.colors['primary'],
                           background='#f0f0f0')
                           
        self.style.configure('Subtitle.TLabel',
                           font=('Segoe UI', 12),
                           foreground=self.colors['dark'],
                           background='#f0f0f0')
                           
    def create_variables(self):
        """Inicializar variables"""
        self.db_path = ""
        self.planilla_path = ""
        self.save_path = ""
        
    def create_widgets(self):
        """Crear widgets compactos sin scroll"""
        # Contenedor principal simple (sin scroll)
        main_frame = tk.Frame(self.window, bg='#f0f0f0', padx=30, pady=20)  # Menos padding
        main_frame.pack(fill='both', expand=True)
        
        # Secciones
        self.create_header(main_frame)
        self.create_cards_section(main_frame)
        self.create_action_section(main_frame)
        self.create_footer(main_frame)
        
        # Establecer estado inicial del botón
        self.check_ready_state()
        
    def create_header(self, parent):
        """Crear encabezado compacto"""
        header_frame = tk.Frame(parent, bg='#f0f0f0')
        header_frame.pack(fill='x', pady=(0, 25))  # Menos espacio
        
        # Título
        title_label = ttk.Label(header_frame, text=f"PyMetrados v{APP_VERSION}", style='Title.TLabel')
        title_label.pack()
        
        # Subtítulo
        subtitle_label = ttk.Label(header_frame, 
                                 text="Herramienta Profesional para Cálculos de Metrados de Construcción",
                                 style='Subtitle.TLabel')
        subtitle_label.pack(pady=(3, 0))  # Menos espacio
        
        # Estado compacto
        status_frame = tk.Frame(header_frame, bg='#f0f0f0')
        status_frame.pack(pady=(8, 0))  # Menos espacio
        
        analytics_status = "Analíticas: ✅ Mixpanel" if ANALYTICS_ENABLED else "Analíticas: ❌ Deshabilitado"
        analytics_label = tk.Label(status_frame, 
                                  text=analytics_status,
                                  font=('Segoe UI', 7), bg='#f0f0f0', fg='#666')  # Fuente más pequeña
        analytics_label.pack()
        
        updates_enabled = config.get("features", {}).get("update_notifications", True)
        updates_status = "Actualizaciones: ✅ Al iniciar" if updates_enabled else "Actualizaciones: ❌ Deshabilitado"
        updates_label = tk.Label(status_frame, 
                                text=updates_status,
                                font=('Segoe UI', 7), bg='#f0f0f0', fg='#666')  # Fuente más pequeña
        updates_label.pack()
        
        # Línea separadora
        separator = tk.Frame(header_frame, height=2, bg=self.colors['primary'])
        separator.pack(fill='x', pady=(15, 0))  # Menos espacio
        
    def create_cards_section(self, parent):
        """Crear sección de tarjetas compacta"""
        cards_frame = tk.Frame(parent, bg='#f0f0f0')
        cards_frame.pack(fill='x', pady=(0, 20))  # Menos espacio
        
        # Tarjetas de archivos más compactas
        self.create_file_card(cards_frame, "Archivo de Configuración", 
                            "Cargar archivo Excel de configuración",
                            "📋", self.open_db, 0)
        
        self.create_file_card(cards_frame, "Archivo de Planilla", 
                            "Cargar archivo Excel de planilla",
                            "📊", self.open_planilla, 1)
        
        self.create_file_card(cards_frame, "Carpeta de Exportación", 
                            "Seleccionar directorio de salida",
                            "💾", self.save_file, 2)
                            
    def create_file_card(self, parent, title, description, icon, command, row):
        """Crear tarjeta individual compacta"""
        card_frame = tk.Frame(parent, bg='white', relief='solid', bd=1, padx=2, pady=2)
        card_frame.pack(fill='x', pady=4)  # Menos espacio entre tarjetas
        
        # Efectos hover
        def on_enter(e):
            card_frame.configure(bg='#e3f2fd')
            
        def on_leave(e):
            card_frame.configure(bg='white')
            
        card_frame.bind("<Enter>", on_enter)
        card_frame.bind("<Leave>", on_leave)
        
        # Contenido compacto
        content_frame = tk.Frame(card_frame, bg='white', padx=15, pady=10)  # Menos padding
        content_frame.pack(fill='both', expand=True)
        
        # Lado izquierdo
        left_frame = tk.Frame(content_frame, bg='white')
        left_frame.pack(side='left', fill='both', expand=True)
        
        # Ícono y título
        title_frame = tk.Frame(left_frame, bg='white')
        title_frame.pack(fill='x')
        
        icon_label = tk.Label(title_frame, text=icon, font=('Segoe UI', 14), 
                             bg='white', fg=self.colors['primary'])  # Ícono más pequeño
        icon_label.pack(side='left', padx=(0, 8))
        
        title_label = tk.Label(title_frame, text=title, 
                              font=('Segoe UI', 11, 'bold'),  # Fuente más pequeña
                              bg='white', fg=self.colors['dark'])
        title_label.pack(side='left')
        
        # Descripción
        desc_label = tk.Label(left_frame, text=description,
                             font=('Segoe UI', 9),  # Fuente más pequeña
                             bg='white', fg='#6c757d')
        desc_label.pack(anchor='w', pady=(3, 0))  # Menos espacio
        
        # Estado
        if row == 0:
            self.db_status = tk.Label(left_frame, text="Ningún archivo seleccionado",
                                     font=('Segoe UI', 8, 'italic'),  # Fuente más pequeña
                                     bg='white', fg=self.colors['warning'])
            self.db_status.pack(anchor='w', pady=(3, 0))
        elif row == 1:
            self.planilla_status = tk.Label(left_frame, text="Ningún archivo seleccionado",
                                           font=('Segoe UI', 8, 'italic'),  # Fuente más pequeña
                                           bg='white', fg=self.colors['warning'])
            self.planilla_status.pack(anchor='w', pady=(3, 0))
        else:
            self.save_status = tk.Label(left_frame, text="Ninguna carpeta seleccionada",
                                       font=('Segoe UI', 8, 'italic'),  # Fuente más pequeña
                                       bg='white', fg=self.colors['warning'])
            self.save_status.pack(anchor='w', pady=(3, 0))
        
        # Botón compacto
        button_text = "Examinar" if row < 2 else "Seleccionar"
        action_btn = tk.Button(content_frame, text=button_text,
                              font=('Segoe UI', 9, 'bold'),  # Fuente más pequeña
                              bg=self.colors['primary'], fg='white',
                              border=0, padx=15, pady=6,  # Menos padding
                              cursor='hand2',
                              command=command)
        action_btn.pack(side='right', padx=(10, 0))
        
        # Efectos hover del botón
        def btn_on_enter(e):
            action_btn.configure(bg=self.colors['secondary'])
            
        def btn_on_leave(e):
            action_btn.configure(bg=self.colors['primary'])
            
        action_btn.bind("<Enter>", btn_on_enter)
        action_btn.bind("<Leave>", btn_on_leave)
        
    def create_action_section(self, parent):
        """Crear sección de acción compacta"""
        action_frame = tk.Frame(parent, bg='#f0f0f0')
        action_frame.pack(fill='x', pady=25)  # Menos espacio
        
        # Sección de progreso (inicialmente oculta)
        self.progress_frame = tk.Frame(action_frame, bg='#f0f0f0')
        self.progress_label = tk.Label(self.progress_frame, 
                                      text="Procesando metrados...",
                                      font=('Segoe UI', 11),  # Fuente más pequeña
                                      bg='#f0f0f0', fg=self.colors['dark'])
        self.progress_label.pack()
        
        self.progress_bar = ttk.Progressbar(self.progress_frame, 
                                          mode='indeterminate',
                                          length=350)  # Más pequeña
        self.progress_bar.pack(pady=(8, 0))
        
        # Mensaje de estado compacto
        self.status_message = tk.Label(action_frame, 
                                      text="⚠️ Seleccione todos los archivos para continuar",
                                      font=('Segoe UI', 10, 'bold'),  # Fuente más pequeña
                                      bg='#f0f0f0', fg=self.colors['warning'])
        self.status_message.pack(pady=(0, 18))  # Menos espacio
        
        # BOTÓN PRINCIPAL COMPACTO
        self.start_button = tk.Button(action_frame, 
                                     text="🚀 INICIAR PROCESAMIENTO",
                                     font=('Segoe UI', 12, 'bold'),  # Fuente más pequeña
                                     bg=self.colors['danger'],
                                     fg='white',
                                     bd=0,
                                     relief='flat',
                                     padx=25,                   # Menos padding
                                     pady=12,                   # Menos padding
                                     cursor='hand2',
                                     command=self.start_processing,
                                     activebackground='#c82333',
                                     activeforeground='white')
        self.start_button.pack(pady=15)  # Menos espacio
        
        # Efectos hover del botón
        def on_enter(e):
            if self.is_ready_to_process():
                self.start_button.configure(bg='#218838')
            else:
                self.start_button.configure(bg='#c82333')
                
        def on_leave(e):
            if self.is_ready_to_process():
                self.start_button.configure(bg=self.colors['success'])
            else:
                self.start_button.configure(bg=self.colors['danger'])
                
        self.start_button.bind("<Enter>", on_enter)
        self.start_button.bind("<Leave>", on_leave)
    
    def create_footer(self, parent):
        """Crear pie de página compacto"""
        footer_frame = tk.Frame(parent, bg='#f0f0f0')
        footer_frame.pack(side='top', fill='x', pady=(20, 0))  # Menos espacio
        
        # Línea separadora
        separator = tk.Frame(footer_frame, height=1, bg='#dee2e6')
        separator.pack(fill='x', pady=(0, 10))  # Menos espacio
        
        footer_text = f"💻 Desarrollado por: www.angelhuayas.com | PyMetrados v{APP_VERSION}"
        if ANALYTICS_ENABLED:
            footer_text += f" | Sesión: {self.session_id}"
        
        footer_label = tk.Label(footer_frame, 
                               text=footer_text,
                               font=('Segoe UI', 8),  # Fuente más pequeña
                               bg='#f0f0f0', fg='#6c757d')
        footer_label.pack()
    
    def open_db(self):
        """Seleccionar archivo de configuración"""
        file_path = filedialog.askopenfilename(
            title="Seleccionar Archivo de Configuración",
            filetypes=[("Archivos Excel", "*.xlsx"), ("Todos los archivos", "*.*")]
        )
        if file_path:
            self.db_path = file_path
            filename = os.path.basename(file_path)
            self.db_status.config(text=f"✓ {filename}", fg=self.colors['success'])
            self.check_ready_state()
            
            track_event('Archivo_Seleccionado', {
                'user_id': self.session_id,
                'tipo': 'configuracion'
            })
            
    def open_planilla(self):
        """Seleccionar archivo de planilla"""
        file_path = filedialog.askopenfilename(
            title="Seleccionar Archivo de Planilla",
            filetypes=[("Archivos Excel", "*.xlsx"), ("Todos los archivos", "*.*")]
        )
        if file_path:
            self.planilla_path = file_path
            filename = os.path.basename(file_path)
            self.planilla_status.config(text=f"✓ {filename}", fg=self.colors['success'])
            self.check_ready_state()
            
            track_event('Archivo_Seleccionado', {
                'user_id': self.session_id,
                'tipo': 'planilla'
            })
            
    def save_file(self):
        """Seleccionar carpeta de exportación"""
        directory = filedialog.askdirectory(title="Seleccionar Carpeta de Exportación")
        if directory:
            self.save_path = directory
            self.save_status.config(text=f"✓ {os.path.basename(directory)}", fg=self.colors['success'])
            self.check_ready_state()
            
            track_event('Carpeta_Seleccionada', {
                'user_id': self.session_id
            })
    
    def is_ready_to_process(self):
        """Verificar si está listo para procesar"""
        return all([self.db_path, self.planilla_path, self.save_path])
            
    def check_ready_state(self):
        """Verificar estado y habilitar/deshabilitar botón"""
        if self.is_ready_to_process():
            # LISTO - Verde elegante
            self.start_button.configure(
                bg=self.colors['success'],  # Verde profesional
                fg='white',
                state='normal',
                cursor='hand2'
            )
            self.status_message.configure(
                text="✅ ¡Todo listo! Haga clic para iniciar el procesamiento",
                fg=self.colors['success']
            )
        else:
            # NO LISTO - Rojo elegante
            self.start_button.configure(
                bg=self.colors['danger'],   # Rojo profesional
                fg='white',
                state='normal',
                cursor='hand2'
            )
            
            # Determinar qué falta
            missing = []
            if not self.db_path:
                missing.append("archivo de configuración")
            if not self.planilla_path:
                missing.append("archivo de planilla") 
            if not self.save_path:
                missing.append("carpeta de exportación")
            
            self.status_message.configure(
                text=f"⚠️ Falta: {', '.join(missing)}",
                fg=self.colors['warning']
            )
    
    def start_processing(self):
        """Iniciar procesamiento"""
        # Verificar que todo esté listo
        if not self.is_ready_to_process():
            messagebox.showerror("Información Faltante", 
                               "Por favor seleccione todos los archivos y carpeta requeridos antes de iniciar.")
            return
        
        # Analytics
        start_time = datetime.now()
        track_event('Procesamiento_Iniciado', {
            'user_id': self.session_id,
            'hora': start_time.isoformat()
        })
            
        # Ocultar botón y mostrar progreso
        self.start_button.pack_forget()
        self.status_message.pack_forget()
        self.progress_frame.pack(pady=20)
        self.progress_bar.start(10)
        self.window.update()
        
        try:
            # Ejecutar script principal
            result = scrip_metrados.metados_py(self.planilla_path, self.db_path, self.save_path)
            
            # Calcular tiempo
            processing_time = (datetime.now() - start_time).total_seconds()
            
            # Restaurar interfaz
            self.progress_bar.stop()
            self.progress_frame.pack_forget()
            self.status_message.pack(pady=(0, 20))
            self.start_button.pack()
            
            if result is True:
                track_event('Procesamiento_Exitoso', {
                    'user_id': self.session_id,
                    'tiempo_segundos': processing_time
                })
                
                messagebox.showinfo("¡Éxito! 🎉", 
                                  "¡Metrado.xlsx se ha generado exitosamente!\n\n"
                                  f"Archivo guardado en: {self.save_path}\n"
                                  f"Tiempo de procesamiento: {processing_time:.1f} segundos")
            else:
                track_event('Procesamiento_Error', {
                    'user_id': self.session_id,
                    'error': str(result)[:100]
                })
                
                messagebox.showerror("Error ❌", f"Ocurrió un error:\n\n{result}")
                
        except Exception as e:
            # Restaurar interfaz en caso de error
            self.progress_bar.stop()
            self.progress_frame.pack_forget()
            self.status_message.pack(pady=(0, 20))
            self.start_button.pack()
            
            track_event('Procesamiento_Excepción', {
                'user_id': self.session_id,
                'excepcion': str(e)[:100]
            })
            
            messagebox.showerror("Error Inesperado", f"Ocurrió un error inesperado:\n\n{str(e)}")
    
    def on_closing(self):
        """Manejar cierre de aplicación"""
        track_event('App_Cerrado', {
            'user_id': self.session_id
        })
        self.window.destroy()
            
    def run(self):
        """Iniciar aplicación"""
        self.window.protocol("WM_DELETE_WINDOW", self.on_closing)
        self.window.mainloop()

# Crear y ejecutar aplicación
if __name__ == "__main__":
    app = PyMetradosApp()
    app.run() 