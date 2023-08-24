import { FactorData } from "./App";

export default function InspectFactorData({
  fdIndex,
  fdDate,
  factorData
}: {
  fdIndex: number | null;
  fdDate: string | null;
  factorData: FactorData[];
}) {
  if (fdIndex === null || fdDate === null || factorData.length === 0) {
    return null;
  }
  const fdDetails = factorData[fdIndex];
  const fdData = fdDetails.data[fdDate];
  return <div>
    <h2>{"Factor: " + fdDetails.name}</h2>
    <h4>{fdDate}</h4>
    <p>{fdDetails.expression}</p>
    {JSON.stringify(fdData)}
  </div>
}