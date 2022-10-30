import tkinter as tk
from tkinter import ttk, messagebox
import json
import os
from datetime import datetime, timedelta
import matplotlib.pyplot as plt
import matplotlib.dates as mdates
from matplotlib.backends.backend_tkagg import FigureCanvasTkAgg
import requests

class AnalyticsDashboard:
    """Dashboard para visualizar analíticas de PyMetrados"""
    
    def __init__(self):
        self.window = tk.Tk()
        self.window.title("PyMetrados Analytics Dashboard")
        self.window.geometry("1200x800")
        self.window.configure(bg='#f5f5f5')
        
        self.setup_ui()
        self.load_data()
        
    def setup_ui(self):
        """Configurar interfaz de usuario"""
        # Header
        header_frame = tk.Frame(self.window, bg='#2E86AB', height=80)
        header_frame.pack(fill='x')
        header_frame.pack_propagate(False)
        
        title_label = tk.Label(header_frame, text="PyMetrados Analytics Dashboard", 
                              font=('Segoe UI', 20, 'bold'), 
                              bg='#2E86AB', fg='white')
        title_label.pack(pady=20)
        
        # Main content
        main_frame = tk.Frame(self.window, bg='#f5f5f5')
        main_frame.pack(fill='both', expand=True, padx=20, pady=20)
        
        # Notebook for tabs
        self.notebook = ttk.Notebook(main_frame)
        self.notebook.pack(fill='both', expand=True)
        
        # Tabs
        self.create_overview_tab()
        self.create_system_tab()
        self.create_usage_tab()
        self.create_performance_tab()
        
    def create_overview_tab(self):
        """Tab de resumen general"""
        overview_frame = ttk.Frame(self.notebook)
        self.notebook.add(overview_frame, text="Overview")
        
        # Métricas principales
        metrics_frame = tk.Frame(overview_frame, bg='white')
        metrics_frame.pack(fill='x', padx=10, pady=10)
        
        self.create_metric_card(metrics_frame, "Total Users", "1,247", "#28a745", 0, 0)
        self.create_metric_card(metrics_frame, "Active Sessions", "89", "#007bff", 0, 1)
        self.create_metric_card(metrics_frame, "Files Processed", "3,456", "#ffc107", 0, 2)
        self.create_metric_card(metrics_frame, "Success Rate", "94.2%", "#17a2b8", 0, 3)
        
        # Gráfico de sesiones por día
        chart_frame = tk.Frame(overview_frame, bg='white')
        chart_frame.pack(fill='both', expand=True, padx=10, pady=10)
        
        self.create_sessions_chart(chart_frame)
        
    def create_system_tab(self):
        """Tab de información de sistemas"""
        system_frame = ttk.Frame(self.notebook)
        self.notebook.add(system_frame, text="System Info")
        
        # Distribución de SO
        os_frame = tk.LabelFrame(system_frame, text="Operating Systems", padx=10, pady=10)
        os_frame.pack(fill='x', padx=10, pady=5)
        
        os_data = [
            ("Windows 10", "67%", "#0078d4"),
            ("Windows 11", "23%", "#005a9e"),
            ("Linux", "8%", "#e95420"),
            ("macOS", "2%", "#000000")
        ]
        
        for i, (os_name, percentage, color) in enumerate(os_data):
            self.create_progress_bar(os_frame, os_name, percentage, color, i)
            
        # Hardware stats
        hw_frame = tk.LabelFrame(system_frame, text="Hardware Statistics", padx=10, pady=10)
        hw_frame.pack(fill='both', expand=True, padx=10, pady=5)
        
        hw_stats = [
            ("Average RAM", "8.4 GB"),
            ("Average CPU Cores", "6.2"),
            ("Most Common Resolution", "1920x1080"),
            ("Python Version", "3.9.7")
        ]
        
        for i, (stat_name, value) in enumerate(hw_stats):
            stat_label = tk.Label(hw_frame, text=f"{stat_name}: {value}", 
                                 font=('Segoe UI', 11), anchor='w')
            stat_label.grid(row=i//2, column=i%2, sticky='w', padx=10, pady=5)
    
    def create_usage_tab(self):
        """Tab de patrones de uso"""
        usage_frame = ttk.Frame(self.notebook)
        self.notebook.add(usage_frame, text="Usage Patterns")
        
        # Horas más activas
        hours_frame = tk.LabelFrame(usage_frame, text="Peak Usage Hours", padx=10, pady=10)
        hours_frame.pack(fill='x', padx=10, pady=5)
        
        peak_hours = ["9:00 AM", "11:00 AM", "2:00 PM", "4:00 PM", "8:00 PM"]
        for i, hour in enumerate(peak_hours):
            hour_label = tk.Label(hours_frame, text=f"{i+1}. {hour}", 
                                 font=('Segoe UI', 10))
            hour_label.pack(anchor='w', pady=2)
            
        # Tipos de archivo más procesados
        files_frame = tk.LabelFrame(usage_frame, text="File Types", padx=10, pady=10)
        files_frame.pack(fill='x', padx=10, pady=5)
        
        file_types = [
            ("Config Files", "45%"),
            ("Planilla Excel", "35%"),
            ("Custom Templates", "20%")
        ]
        
        for file_type, percentage in file_types:
            type_label = tk.Label(files_frame, text=f"{file_type}: {percentage}", 
                                 font=('Segoe UI', 10))
            type_label.pack(anchor='w', pady=2)
    
    def create_performance_tab(self):
        """Tab de métricas de rendimiento"""
        perf_frame = ttk.Frame(self.notebook)
        self.notebook.add(perf_frame, text="Performance")
        
        # Tiempo promedio de procesamiento
        time_frame = tk.LabelFrame(perf_frame, text="Processing Times", padx=10, pady=10)
        time_frame.pack(fill='x', padx=10, pady=5)
        
        time_stats = [
            ("Average Processing Time", "12.4 seconds"),
            ("Fastest Process", "3.2 seconds"),
            ("Slowest Process", "45.6 seconds"),
            ("95th Percentile", "23.1 seconds")
        ]
        
        for stat_name, value in time_stats:
            stat_label = tk.Label(time_frame, text=f"{stat_name}: {value}", 
                                 font=('Segoe UI', 11))
            stat_label.pack(anchor='w', pady=2)
            
        # Error rates
        error_frame = tk.LabelFrame(perf_frame, text="Error Analysis", padx=10, pady=10)
        error_frame.pack(fill='x', padx=10, pady=5)
        
        error_types = [
            ("File Not Found", "2.3%", "#dc3545"),
            ("Permission Error", "1.8%", "#fd7e14"),
            ("Format Error", "1.2%", "#ffc107"),
            ("Memory Error", "0.4%", "#6f42c1")
        ]
        
        for error_type, percentage, color in error_types:
            self.create_progress_bar(error_frame, error_type, percentage, color, len(error_types))
    
    def create_metric_card(self, parent, title, value, color, row, col):
        """Crear tarjeta de métrica"""
        card_frame = tk.Frame(parent, bg=color, relief='raised', bd=2)
        card_frame.grid(row=row, column=col, padx=10, pady=10, sticky='ew')
        
        parent.grid_columnconfigure(col, weight=1)
        
        title_label = tk.Label(card_frame, text=title, 
                              font=('Segoe UI', 10), 
                              bg=color, fg='white')
        title_label.pack(pady=(10, 0))
        
        value_label = tk.Label(card_frame, text=value, 
                              font=('Segoe UI', 20, 'bold'), 
                              bg=color, fg='white')
        value_label.pack(pady=(0, 10))
    
    def create_progress_bar(self, parent, label, percentage, color, row):
        """Crear barra de progreso con etiqueta"""
        label_frame = tk.Frame(parent)
        label_frame.pack(fill='x', pady=2)
        
        name_label = tk.Label(label_frame, text=label, font=('Segoe UI', 10))
        name_label.pack(side='left')
        
        percent_label = tk.Label(label_frame, text=percentage, font=('Segoe UI', 10, 'bold'))
        percent_label.pack(side='right')
        
        progress = ttk.Progressbar(parent, length=300, mode='determinate')
        progress.pack(fill='x', pady=(0, 5))
        progress['value'] = float(percentage.strip('%'))
    
    def create_sessions_chart(self, parent):
        """Crear gráfico de sesiones"""
        try:
            import matplotlib.pyplot as plt
            from matplotlib.backends.backend_tkagg import FigureCanvasTkAgg
            
            fig, ax = plt.subplots(figsize=(10, 4))
            
            # Datos de ejemplo
            dates = [datetime.now() - timedelta(days=i) for i in range(7, 0, -1)]
            sessions = [45, 52, 48, 67, 59, 73, 89]
            
            ax.plot(dates, sessions, marker='o', linewidth=2, markersize=6, color='#2E86AB')
            ax.fill_between(dates, sessions, alpha=0.3, color='#2E86AB')
            
            ax.set_title('Daily Sessions (Last 7 Days)', fontsize=14, fontweight='bold')
            ax.set_ylabel('Sessions', fontsize=12)
            ax.grid(True, alpha=0.3)
            
            # Formatear fechas
            ax.xaxis.set_major_formatter(mdates.DateFormatter('%m/%d'))
            plt.xticks(rotation=45)
            
            plt.tight_layout()
            
            canvas = FigureCanvasTkAgg(fig, parent)
            canvas.draw()
            canvas.get_tk_widget().pack(fill='both', expand=True)
            
        except ImportError:
            # Fallback si matplotlib no está disponible
            fallback_label = tk.Label(parent, text="Chart requires matplotlib\nInstall with: pip install matplotlib", 
                                     font=('Segoe UI', 12), fg='gray')
            fallback_label.pack(expand=True)
    
    def load_data(self):
        """Cargar datos de analíticas"""
        # Aquí cargarías datos reales desde tu base de datos o API
        # Por ahora usamos datos de ejemplo
        pass
    
    def export_report(self):
        """Exportar reporte de analíticas"""
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
        filename = f"pymetrados_analytics_{timestamp}.json"
        
        report_data = {
            "timestamp": datetime.now().isoformat(),
            "period": "last_30_days",
            "metrics": {
                "total_users": 1247,
                "active_sessions": 89,
                "files_processed": 3456,
                "success_rate": 94.2
            },
            "system_info": {
                "windows_10": 67,
                "windows_11": 23,
                "linux": 8,
                "macos": 2
            },
            "performance": {
                "avg_processing_time": 12.4,
                "error_rate": 5.8
            }
        }
        
        try:
            with open(filename, 'w') as f:
                json.dump(report_data, f, indent=2)
            messagebox.showinfo("Export Success", f"Report exported to {filename}")
        except Exception as e:
            messagebox.showerror("Export Error", f"Failed to export: {e}")
    
    def run(self):
        """Ejecutar dashboard"""
        # Botón de exportar
        export_btn = tk.Button(self.window, text="Export Report", 
                              command=self.export_report,
                              font=('Segoe UI', 10, 'bold'),
                              bg='#28a745', fg='white',
                              border=0, padx=20, pady=5)
        export_btn.place(relx=0.02, rely=0.94)
        
        self.window.mainloop()

if __name__ == "__main__":
    try:
        dashboard = AnalyticsDashboard()
        dashboard.run()
    except Exception as e:
        print(f"Error running dashboard: {e}")
        print("Install required packages: pip install matplotlib") 