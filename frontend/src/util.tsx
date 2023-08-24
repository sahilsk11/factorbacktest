import { FactorData } from './App';

export function enumerateDates(startDateStr: string, endDateStr: string) {
  const startDate = new Date(startDateStr+ "T00:00:00");
  const endDate = new Date(endDateStr+ "T00:00:00");

  const dates: string[] = [];
  const currentDate = new Date(startDate);

  while (currentDate <= endDate) {
    dates.push(formatDate(currentDate));
    currentDate.setDate(currentDate.getDate() + 1);
  }

  return dates;
}

export function formatDate(date: Date) {
  const year = date.getFullYear();
  const month = (date.getMonth() + 1).toString().padStart(2, '0');
  const day = date.getDate().toString().padStart(2, '0');

  return `${year}-${month}-${day}`;
}

export const minMaxDates = (factorData:FactorData[]): {min: string; max:string} => {
  let min = "";
  let max = ""

  factorData.forEach(fd => {
    Object.keys(fd.data).forEach(date => {
      if (min === "" || date < min) {
        min = date;
      }
      if (max === "" || date > max) {
        max = date;
      }
    })
  })

  return {min, max};
}