import tkinter as tk
from tkinter import ttk, filedialog, messagebox
from PIL import Image, ImageTk
import os
import scrip_metrados
import sys
from mixpanel import Mixpanel

# Crea una instancia de Mixpanel con tu token de proyecto
mp = Mixpanel('3f89c70f58f582dfc898380a1699e843')

# Rastrea un evento con un nombre y propiedades
mp.track('distinct_id', 'Start Session', {'Property Name': 'Pymetrados'})

# Create an instance of tkinter frame
win = tk.Tk()

# Set the title and geometry, and prevent resizing
win.title('PyMetrados')
win.geometry("750x550")
win.resizable(0, 0)
win.columnconfigure(0, weight=1)
win.columnconfigure(4, weight=1)
# Set the style for the widgets
style = ttk.Style()
style.theme_use('clam')
style.configure('TButton', font=('Arial', 12), borderwidth=1)
style.configure('TLabel', font=('Arial', 12), foreground='#0F1FFF')
style.configure('Header.TLabel', font=('Arial', 18, 'bold'))


# Load an image for the background
bundle_dir = getattr(sys, '_MEIPASS', os.path.abspath(os.path.dirname(__file__)))
path_to_yml = os.path.abspath(os.path.join(bundle_dir, 'cover.jpg'))

bg_image = Image.open(path_to_yml)
bg_photo = ImageTk.PhotoImage(bg_image.resize((750, 550), Image.LANCZOS))
bg_label = tk.Label(win, image=bg_photo)
bg_label.place(x=0, y=0, relwidth=1, relheight=1)

# Global paths
db_path = ""
planilla_path = ""
save_path = ""

# Define functions for opening files and directories
def open_db():
    global db_path
    db_path = filedialog.askopenfilename(filetypes=[("Excel Files", "*.xlsx")])
    if db_path:
        db_label.config(text=f"Config File: {os.path.basename(db_path)}")

def open_planilla():
    global planilla_path
    planilla_path = filedialog.askopenfilename(filetypes=[("Excel Files", "*.xlsx")])
    if planilla_path:
        planilla_label.config(text=f"Planilla File: {os.path.basename(planilla_path)}")

def save_file():
    global save_path
    save_path = filedialog.askdirectory()
    if save_path:
        save_label.config(text=f"Save Location: {save_path}")


def start():
    # Check if all paths are defined
    if not all([db_path, planilla_path, save_path]):
        messagebox.showerror("Error", "Please select all files and the save location before starting.")
        return
    # Execute the main script
    result = scrip_metrados.metados_py(planilla_path, db_path, save_path)
    if result is True:
        messagebox.showinfo("Success", "Metrado.xlsx has been generated successfully.")
    else:
        messagebox.showerror("Error", result)

# Add header label
header_label = ttk.Label(win, text="PYMETRADOS", style='Header.TLabel')
header_label.grid(row=1, column=1, columnspan=2, pady=20)

# Add labels for file paths
db_label = ttk.Label(win, text="No Config File Selected", style='TLabel')
planilla_label = ttk.Label(win, text="No Planilla File Selected", style='TLabel')
save_label = ttk.Label(win, text="No Save Location Selected", style='TLabel')


db_label.place(relx=0.75, rely=0.3, anchor='center')
planilla_label.place(relx=0.75, rely=0.4, anchor='center')
save_label.place(relx=0.75, rely=0.5, anchor='center')

# Add buttons for file dialogs
config_button = ttk.Button(win, text="Load Config", command=open_db)
planilla_button = ttk.Button(win, text="Load Planilla", command=open_planilla)
save_button = ttk.Button(win, text="Save to", command=save_file)
start_button = ttk.Button(win, text="Start", command=start)

config_button.place(relx=0.25, rely=0.3, anchor='center')
planilla_button.place(relx=0.25, rely=0.4, anchor='center')
save_button.place(relx=0.25, rely=0.5, anchor='center')
start_button.place(relx=0.25, rely=0.6, anchor='center')

# Add footer label
footer_label = ttk.Label(win, text="Developed by: www.angelhuayas.com", style='TLabel')
footer_label.place(relx=0.5, rely=0.9, anchor='center')

# Start the GUI event loop
win.mainloop()
