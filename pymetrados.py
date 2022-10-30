# Import the required Libraries
from distutils.command.config import config
from mimetypes import init
from tkinter import *
import tkinter as tk
from tkinter import ttk, filedialog
import scrip_metrados
from tkinter.filedialog import askopenfile
import os
from PIL import Image, ImageTk
import sys

# Create an instance of tkinter frame
win = Tk()

# Set the geometry of tkinter frame
win.geometry("650x450")
win.title('PÿMetrados')
win.resizable(0,0)
bundle_dir = getattr(sys, '_MEIPASS', os.path.abspath(os.path.dirname(__file__)))
path_to_yml = os.path.abspath(os.path.join(bundle_dir, 'cover.jpg'))
image = Image.open(path_to_yml)
# jewimage = image.resize((650,450), Image.ANTIALIAS)
# jewimage.save("cover.jpg")
width, height = win.winfo_screenwidth(), win.winfo_screenheight()

image = ImageTk.PhotoImage(image)
bg_label = ttk.Label(win, image = image)
bg_label.place(x=0, y=0, relwidth=1, relheight=1)

db_path = ""
planilla_path = ""
save_path = ""

def open_db():
   file = filedialog.askopenfilename(filetypes=[("Configuration","*.xlsx")])
   if file:
      filepath = os.path.abspath(file)
      global db_path
      db_path =  filepath
      open_label = ttk.Label(win, text="La configuración esta en: " + str(filepath), background='black', foreground='#00FFFF', font=('Aerial 8'))
      open_label.grid(column=1, row=1, sticky=tk.W, padx=5, pady=5)
def open_planilla():
   file = filedialog.askopenfilename(filetypes=[("Planilla","*.xlsx")])
   if file:
      filepath = os.path.abspath(file)
      global planilla_path
      planilla_path = filepath
      planilla_label = ttk.Label(win, text="La planilla esta en: " + str(filepath), background='black', foreground='#00FFFF', font=('Aerial 8'))
      planilla_label.grid(column=1, row=2, sticky=tk.W, padx=5, pady=5)

def save_file():
   file = filedialog.askdirectory()
   if file:
      filepath = os.path.abspath(file)
      global save_path
      save_path =  filepath
      save_label = ttk.Label(win, text="Exportar en : " + str(filepath), background='black', foreground='#00FFFF', font=('Aerial 8'))
      save_label.grid(column=1, row=3, sticky=tk.W, padx=5, pady=5)
# Add a Label widget



def start():
   text = tk.StringVar()
   finish_label =  Label(win, textvariable=text, background='black', foreground='#FF0000', font=('Aerial 12'))

   if db_path and planilla_path and save_path:
      finish = scrip_metrados.metados_py(planilla_path, db_path, save_path)

      if finish:
         text.set("Se genero Metrado.xlsx, Tarea Finalizada")
         finish_label.grid(column=1, row=4, sticky=tk.W, padx=5, pady=5)
   else:
      if db_path:
         text.set("Debe de seleccionar la ubicacion de la planilla")
         finish_label.grid(column=1, row=4, sticky=tk.W, padx=5, pady=5)
      else:
         text.set("Debe de seleccionar la base de datos")
         finish_label.grid(column=1, row=4, sticky=tk.W, padx=5, pady=5)


  
# Show image using label
style = ttk.Style()
style.theme_use('alt')
win.columnconfigure(0, weight=1)
win.columnconfigure(1, weight=3)
title =  Label(win, text="PYMETRADOS", background='black', foreground='#00FFFF', font=('Aerial 15'))
title.grid(columnspan=2, row=0, padx=5, pady=20)
footer =  Label(win, text="Desarrollado por: www.angelhuayas.com", background='black', foreground='#00FFFF', font=('Aerial 8'))
footer.grid(columnspan=2, row=5, padx=5, pady=20)
style.configure('TButton', background = '#00FFFF', foreground = 'black', width = 20, borderwidth=1, focusthickness=3, focuscolor='none')
style.map('TButton', background=[('active','black')], foreground = [('active','white')] )

config_button = ttk.Button(win, text="Cargar config", command=open_db, style='Fun.TButton')
planilla_button = ttk.Button(win, text="Cargar planilla", command=open_planilla)
save_button = ttk.Button(win, text="Guardar en:", command=save_file)
start_button = ttk.Button(win, text="Start", command=start)
combo = Text(win, height=8)


config_button.grid(column=0, row=1, padx=5, pady=20)
planilla_button.grid(column=0, row=2, padx=5, pady=20)

save_button.grid(column=0, row=3, padx=5, pady=20)

start_button.grid(column=0, row=4, padx=5, pady=20)

class Frame():
   def __init__(self):
      self = self
   def show_frame(self,win):
      win.mainloop()

if __name__ == "__main__":
   app = Frame()
   app.show_frame(win)     


