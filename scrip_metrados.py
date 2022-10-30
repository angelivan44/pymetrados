from os import X_OK
from numpy import count_nonzero
import openpyxl as xl
from openpyxl import load_workbook
from win32com.client import Dispatch
import win32com
def change_function(arg):
        return arg.value


def fun(arg, data):
    
    if (arg in data):
        return True
    else:
        return False

def sort_func(arg):
    return arg['id']

def get_uniques_values(data):
    count_data = list(map(change_function,data))
    new_set = set(count_data)
    new_set.remove(None)
    new_array = []
    for i in new_set:
        if i != "Id":
            new_array.append( {"id": i, "size" : len(list(filter(lambda x: x == i, count_data))) })  

    return new_array

def metados_py(planilla,db,export):
    #get paths
    planilla_path = planilla
    db_path = db
    result_path = export+'\\Metrado.xlsx'

    #setup openpyxl for win32com

    xl = Dispatch("Excel.Application")
    xl.Visible = False  # You can remove this line if you don't want the Excel application to be visible

    #open workboos
    bd_workbook = xl.Workbooks.Open(Filename=db_path)
    result_workbook = xl.Workbooks.Add()
    datos_workbook = xl.Workbooks.Open(Filename=planilla_path)

    #open worksheets for xl and win32com

    result_worksheet = result_workbook.Worksheets(1)
    planilla_sheet = bd_workbook.Worksheets("P_0")
    metrado_sheet = bd_workbook.Worksheets("M_0")
    planilla_workbook = load_workbook(filename =planilla_path)
    data_base_workbook = load_workbook(filename=db )
    #get worksheets
    data = planilla_workbook["DB"]
    data_headers = planilla_workbook["PLANILLA"]
    XlDirectionDown = 4
    #get data for planilla worksheets
    range_ids = data['A':'A']
    data_general = data_base_workbook["General"]

    range_dates = data_general["A2:E"+str(data_general.max_row)]
    dic_data = {}
    range_dates_general =  list(map(lambda x:  {"id": x[0].value, "name": x[1].value, "code": x[2].value, "system": x[3].value, "location":x[4].value}, list(range_dates)))
    for data in range_dates_general:
        dic_data[data['id']] = data
    range_headers = data_headers['A1':'AZ1'][0]


    #get data for planilla worksheets

    projects_ids = get_uniques_values(range_ids)
    projects_count =  len(projects_ids)
    projects_leng = projects_ids[1]
    count_headers = len(get_uniques_values(range_headers))
    acum = 0
    for item in projects_ids:
        data_project = dic_data[item['id']]
        planilla_sheet.Copy(Before= result_worksheet)
        new_planilla = result_workbook.Worksheets("P_0")
        new_planilla.Name = f"P_{item['id']}"
        new_planilla.Range("F8").Value = data_project['name']
        new_planilla.Range("W4").Value = data_project['code']
        new_planilla.Range("W8").Value = data_project['system']
        new_planilla.Range("W9").Value = data_project['location']
        new_planilla.Range("A16").EntireRow.Offset(1).GetResize(item['size']).Insert(Shift=-4121)
        new_planilla.Range("C10").Value = item['size']
        datos_worksheet = datos_workbook.Worksheets("PLANILLA")
        initial_value = 2 + acum
        acum = initial_value + item['size'] - 2
        last_value = initial_value + item['size'] - 1
        datos_worksheet.Range(datos_worksheet.Cells(initial_value,1), datos_worksheet.Cells(last_value,count_headers)).Copy()
        new_planilla.Range("B15").PasteSpecial(Paste=-4163)

        metrado_sheet.Copy(Before= result_worksheet)
        montaje_data = result_workbook.Worksheets("M_0")
        montaje_data.Name = f"M_{item['id']}"
        montaje_data.Range("A47").Value = data_project['id']
        montaje_data.Range("C47").Value = data_project['name']
        montaje_data.Range(montaje_data.Cells(1,9),montaje_data.Cells(count_headers,item['size']+9)).FormulaArray = "=TRANSPOSE(" +"P_"+str(item['id']) + "!R[14]C[-7]:R[" + str(14+item['size']) + "]C["+ str(count_headers-6) + "])"
        montaje_data.Range(montaje_data.Cells(49,9), montaje_data.Cells(1000,9)).AutoFill(montaje_data.Range(montaje_data.Cells(49,9), montaje_data.Cells(1000,8+item['size'])), 0 )
        
      
    metrado_sheet.Copy(Before= result_workbook.Worksheets(1))
    resumen_planilla = result_workbook.Worksheets("M_0")
    resumen_planilla.Name = "Resumen"
    max_row = data_base_workbook["M_0"].max_row
    for idx, project in enumerate(range_dates_general):
        resumen_planilla.Range("I50").Offset(1,1+idx).Value = project['name']
        resumen_planilla.Range("I50").Offset(2,1+idx).Value = project['id']
        resumen_planilla.Range("I50").Offset(3,1+idx).Formula  = f"=+M_{project['id']}!G52"
    resumen_planilla.Range("I52")
    resumen_planilla.Range(resumen_planilla.Cells(52,9), resumen_planilla.Cells(52,9+len(range_dates_general))).AutoFill(resumen_planilla.Range(resumen_planilla.Cells(52,9), resumen_planilla.Cells(max_row,9+len(range_dates_general))), 0 )
    resumen_planilla.Range(resumen_planilla.Cells(52,7), resumen_planilla.Cells(max_row,7)).Copy()
    resumen_planilla.Range(resumen_planilla.Cells(52,9), resumen_planilla.Cells(max_row,9+len(range_dates_general))).PasteSpecial(Paste=-4122)
    resumen_planilla.Range(resumen_planilla.Cells(49,9), resumen_planilla.Cells(51,9)).Copy()
    resumen_planilla.Range(resumen_planilla.Cells(49,9), resumen_planilla.Cells(51,8 + len(range_dates_general))).PasteSpecial(Paste=-4122)
    resumen_planilla.Range("A1:A45").EntireRow.Delete()
    xl.DisplayAlerts = False

    dummy_sheet = result_workbook.Worksheets("Sheet1")
    dummy_sheet.Delete()
    bd_workbook.Close(SaveChanges=False)
    datos_workbook.Close(SaveChanges=False)
    result_workbook.SaveAs(result_path, ConflictResolution=2)
    result_workbook.Close()
    xl.Quit()
    return True