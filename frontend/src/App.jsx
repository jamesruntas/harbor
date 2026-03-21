import { IndexFolder } from '../wailsjs/go/main/App'


function App() {
    const run = async () => {
            const result = await IndexFolder("C:\\PhoneMedia")
            console.log(result)
     }   
    return <button onClick={run}>Index C:\PhoneMedia</button>
}

export default App
